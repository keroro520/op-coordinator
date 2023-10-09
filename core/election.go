package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/node-real/op-coordinator/config"
	"github.com/node-real/op-coordinator/metrics"
	"github.com/node-real/op-coordinator/types"
)

type Election struct {
	log    log.Logger
	Config config.Config

	activeSequencer string
	Nodes           map[string]*types.Node

	healthChecker HealthChecker

	adminCh chan AdminCommand

	stoppedHash *common.Hash
}

func NewElection(config config.Config, hc HealthChecker, nodes map[string]*types.Node, log log.Logger) *Election {
	return &Election{
		log:           log,
		Config:        config,
		Nodes:         nodes,
		healthChecker: hc,
		adminCh:       make(chan AdminCommand),
	}
}

func (c *Election) StoppedHash() *common.Hash {
	return c.stoppedHash
}

func (c *Election) ActiveSequencerName() string {
	return c.activeSequencer
}

func (c *Election) ActiveSequencer() *types.Node {
	if c.activeSequencer == "" {
		return nil
	}
	return c.Nodes[c.activeSequencer]
}

func (c *Election) AdminCh() chan AdminCommand {
	return c.adminCh
}

func (c *Election) Start(ctx context.Context) {
	c.loop(ctx)
}

func (c *Election) IsSequencer(nodeName string) bool {
	return c.Config.Sequencers[nodeName] != nil
}

func (c *Election) IsHealthy(nodeName string) bool {
	return c.healthChecker.IsHealthy(nodeName)
}

func (c *Election) HealthySequencers() []*types.Node {
	healthy := make([]*types.Node, 0)
	for _, node := range c.Nodes {
		if c.IsSequencer(node.Name) && c.IsHealthy(node.Name) {
			healthy = append(healthy, node)
		}
	}
	return healthy
}

func (c *Election) HealthyNodes() []*types.Node {
	healthy := make([]*types.Node, 0)
	for _, node := range c.Nodes {
		if c.IsHealthy(node.Name) {
			healthy = append(healthy, node)
		}
	}
	return healthy
}

func (c *Election) loop(ctx context.Context) {
	lastActiveFlagCheck := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Set metrics
			activeSequencer := c.ActiveSequencer()
			metrics.MetricElectionEnabled.Set(bool2float64(!c.Config.Election.Stopped))
			metrics.MetricIsActiveSequencer.WithLabelValues("empty").Set(bool2float64(activeSequencer == nil))
			for _, node := range c.Nodes {
				if c.IsSequencer(node.Name) {
					isActive := activeSequencer != nil && activeSequencer.Name == node.Name
					metrics.MetricIsActiveSequencer.WithLabelValues(node.Name).Set(bool2float64(isActive))
				}
			}

			if c.Config.Election.Stopped {
				c.log.Info("Auto-election is disabled, skip", "active sequencer", c.activeSequencer)

			} else if c.ActiveSequencer() == nil {
				err := c.MaybeElect()
				if err != nil {
					c.log.Error("Maybe elect", "error", err)
				}

			} else if c.ActiveSequencer() != nil && !c.IsHealthy(c.activeSequencer) {
				err := c.RevokeActiveSequencer()
				if err != nil {
					c.log.Error("Revoke active sequencer", "error", err)
				}

			} else if c.ActiveSequencer() != nil && lastActiveFlagCheck.Add(3*time.Second).Before(time.Now()) {
				lastActiveFlagCheck = time.Now()
				if active := c.CheckFlagOfActiveSequencer(); !active {
					c.log.Warn("ActiveSequencer is inactive, active it", "active sequencer", c.activeSequencer)
					if err := c.startSequencer(c.Nodes[c.activeSequencer]); err != nil {
						c.log.Error("start sequencer", "node", c.activeSequencer, "error", err)
					}
				}
			}

		case cmd := <-c.adminCh:
			cmd.Execute(c)
		}
	}
}

// CheckSufficientHealthyNodes checks if there are sufficient healthy nodes to elect a new active sequencer.
func (c *Election) CheckSufficientHealthyNodes() bool {
	return len(c.HealthyNodes()) >= c.Config.Election.MinRequiredHealthyNodes
}

func (c *Election) CheckCanonicalContainsStoppedHash(node *types.Node) bool {
	stoppedHash := c.stoppedHash
	if stoppedHash == nil || *stoppedHash == (common.Hash{}) {
		return true
	}

	block, err := node.OpGeth.BlockByHash(context.Background(), *stoppedHash)
	return err == nil && block != nil
}

func (c *Election) CheckFlagOfActiveSequencer() bool {
	activeSequencer := c.ActiveSequencer()
	if activeSequencer == nil {
		return false
	}

	active, err := activeSequencer.OpNode.SequencerActive(context.Background())
	return err == nil && active
}

func (c *Election) findExistingActiveSequencer() *types.Node {
	actives, err := FindActiveNodes(c.HealthySequencers())
	if err != nil {
		return nil
	}

	canonical, err := FindCanonicalNode(actives)
	if err != nil {
		return nil
	}

	return canonical
}

func (c *Election) MaybeElect() error {
	if c.ActiveSequencer() != nil {
		return nil
	}

	c.log.Info("ActiveSequencer is empty, start electing new active sequencer")

	isSufficientHealthyNodes := c.CheckSufficientHealthyNodes()
	if !isSufficientHealthyNodes {
		return errors.New("insufficient healthy nodes")
	}

	canonical, err := c.elect()
	if err != nil || canonical == nil {
		return fmt.Errorf("fail to elect active sequencer, error: %s", err)
	}

	containsStoppedHash := c.CheckCanonicalContainsStoppedHash(canonical)
	if !containsStoppedHash {
		return fmt.Errorf("canonical does not contains prev stopped hash, node: %s, stoppedHash: %s", canonical, c.stoppedHash)
	}

	err = c.SetActiveSequencer(canonical.Name)
	if err != nil {
		return fmt.Errorf("fail to set active sequencer, node: %s, error: %s", canonical.Name, err)
	}

	return nil
}

func (c *Election) elect() (*types.Node, error) {
	c.log.Info("Start election")
	if existingActiveSequencer := c.findExistingActiveSequencer(); existingActiveSequencer != nil {
		return existingActiveSequencer, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.Election.MaxWaitingTimeForConvergenceMs)*time.Millisecond)
	defer cancel()
	WaitForNodesConvergence(c.log, ctx, c.HealthySequencers())

	return FindCanonicalNode(c.HealthySequencers())
}

// RevokeActiveSequencer revokes the leadership of the current active sequencer and returns the stopped hash.
func (c *Election) RevokeActiveSequencer() error {
	activeSequencer := c.ActiveSequencer()
	if activeSequencer == nil {
		return nil
	}

	c.log.Warn("Revoking active sequencer", "active sequencer", c.activeSequencer)

	active, err := activeSequencer.OpNode.SequencerActive(context.Background())
	if err != nil {
		return fmt.Errorf("fail to call admin_sequencerActive, node: %s, error: %s", activeSequencer.Name, err)
	}

	var stopHash common.Hash
	if active {
		respStopHash, err := activeSequencer.OpNode.StopSequencer(context.Background())
		if err != nil {
			return fmt.Errorf("fail to call admin_stopSequencer, node: %s, error: %s", activeSequencer.Name, err)
		}
		if respStopHash == (common.Hash{}) {
			return fmt.Errorf("admin_stopSequencer returns zero hash, node: %s", activeSequencer.Name)
		}
		stopHash = respStopHash
	} else {
		syncStatus, err := activeSequencer.OpNode.SyncStatus(context.Background())
		if err != nil {
			return fmt.Errorf("fail to call optimism_syncStatus, node: %s, error: %s", activeSequencer.Name, err)
		}
		if syncStatus.UnsafeL2.Hash == (common.Hash{}) {
			return fmt.Errorf("optimism_syncStatus returns zero hash, node: %s", activeSequencer.Name)
		}

		stopHash = syncStatus.UnsafeL2.Hash
	}

	c.stoppedHash = &stopHash
	return nil
}

func (c *Election) SetActiveSequencer(nodeName string) error {
	node := c.Nodes[nodeName]
	if node == nil {
		return errors.New("node is not found")
	}
	if !c.IsSequencer(node.Name) {
		return errors.New("node is not a sequencer")
	}
	if c.activeSequencer == node.Name {
		return errors.New("node is already the active sequencer")
	}

	if err := c.startSequencer(node); err != nil {
		return err
	}

	c.log.Info("assign active sequencer", "node", nodeName)
	c.activeSequencer = nodeName
	c.stoppedHash = nil

	return nil
}

func (c *Election) startSequencer(node *types.Node) error {
	if err := node.OpNode.ResetDerivationPipeline(context.Background()); err != nil {
		return fmt.Errorf("fail to call admin_resetDerivationPipeline, node: %s, error: %s", node.Name, err)
	}

	status, err := node.OpNode.SyncStatus(context.Background())
	if err != nil {
		return fmt.Errorf("fail to call optimism_sysStatus, node: %s, error: %s", node.Name, err)
	}

	if err = node.OpNode.StartSequencer(context.Background(), status.UnsafeL2.Hash); err != nil && !strings.Contains(err.Error(), "sequencer already running") {
		return fmt.Errorf("fail to call admin_startSequencer, node: %s, error: %s", node.Name, err)
	}
	return nil
}
