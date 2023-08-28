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

	master string
	Nodes  map[string]*types.Node

	healthChecker HealthChecker

	adminCh chan AdminCommand

	prevStoppedHash *common.Hash
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
	return c.prevStoppedHash
}

func (c *Election) Master() *types.Node {
	if c.master == "" {
		return nil
	}
	master := c.Nodes[c.master]
	return master
}

func (c *Election) AdminCh() chan AdminCommand {
	return c.adminCh
}

func (c *Election) Start(ctx context.Context) {
	c.loop(ctx)
}

func (c *Election) IsCandidate(nodeName string) bool {
	return c.Config.Candidates[nodeName] != nil
}

func (c *Election) IsHealthy(nodeName string) bool {
	return c.healthChecker.IsHealthy(nodeName)
}

func (c *Election) HealthyCandidates() []*types.Node {
	healthy := make([]*types.Node, 0)
	for _, node := range c.Nodes {
		if c.IsCandidate(node.Name) && c.IsHealthy(node.Name) {
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
	lastMasterFlagCheck := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Set metrics
			master := c.Master()
			metrics.MetricElectionEnabled.Set(bool2float64(!c.Config.Election.Stopped))
			metrics.MetricIsMaster.WithLabelValues("empty").Set(bool2float64(master == nil))
			for _, node := range c.Nodes {
				if c.IsCandidate(node.Name) {
					isMaster := master != nil && master.Name == node.Name
					metrics.MetricIsMaster.WithLabelValues(node.Name).Set(bool2float64(isMaster))
				}
			}

			if c.Config.Election.Stopped {
				c.log.Info("Auto-election is disabled, skip", "master", c.master)

			} else if c.Master() == nil {
				err := c.MaybeElect()
				if err != nil {
					c.log.Error("Maybe elect", "error", err)
				}

			} else if c.Master() != nil && !c.IsHealthy(c.master) {
				err := c.RevokeMaster()
				if err != nil {
					c.log.Error("Revoke master", "error", err)
				}

			} else if c.Master() != nil && lastMasterFlagCheck.Add(3*time.Second).Before(time.Now()) {
				lastMasterFlagCheck = time.Now()
				if active := c.CheckMasterIsActive(); !active {
					c.log.Warn("Master is inactive, active it", "master", c.master)
					if err := c.startSequencer(c.Nodes[c.master]); err != nil {
						c.log.Error("start sequencer", "node", c.master, "error", err)
					}
				}
			}

		case cmd := <-c.adminCh:
			cmd.Execute(c)
		}
	}
}

// CheckSufficientHealthyNodes checks if there are sufficient healthy nodes to elect a new master.
func (c *Election) CheckSufficientHealthyNodes() bool {
	return len(c.HealthyNodes()) >= c.Config.Election.MinRequiredHealthyNodes
}

func (c *Election) CheckCanonicalContainsPrevStoppedHash(node *types.Node) bool {
	stoppedHash := c.prevStoppedHash
	if stoppedHash == nil || *stoppedHash == (common.Hash{}) {
		return true
	}

	block, err := node.OpGeth.BlockByHash(context.Background(), *stoppedHash)
	return err == nil && block != nil
}

func (c *Election) CheckMasterIsActive() bool {
	if c.master == "" {
		return false
	}

	master := c.Nodes[c.master]
	active, err := master.OpNode.SequencerActive(context.Background())
	return err == nil && active
}

func (c *Election) findExistingMaster() *types.Node {
	actives, err := FindActiveNodes(c.HealthyCandidates())
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
	if c.Master() != nil {
		return nil
	}

	c.log.Info("Master is empty, start electing new master")

	isSufficientHealthyNodes := c.CheckSufficientHealthyNodes()
	if !isSufficientHealthyNodes {
		return errors.New("insufficient healthy nodes")
	}

	canonical, err := c.elect()
	if err != nil || canonical == nil {
		return fmt.Errorf("fail to elect master, error: %s", err)
	}

	containsStoppedHash := c.CheckCanonicalContainsPrevStoppedHash(canonical)
	if !containsStoppedHash {
		return fmt.Errorf("canonical does not contains prev stopped hash, node: %s, stoppedHash: %s", canonical, c.prevStoppedHash)
	}

	err = c.setMaster(canonical.Name)
	if err != nil {
		return fmt.Errorf("fail to set master, node: %s, error: %s", canonical, err)
	}

	return nil
}

func (c *Election) elect() (*types.Node, error) {
	c.log.Info("Start election")
	if existingMaster := c.findExistingMaster(); existingMaster != nil {
		return existingMaster, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.Election.MaxWaitingTimeForConvergenceMs)*time.Millisecond)
	defer cancel()
	WaitForNodesConvergence(c.log, ctx, c.HealthyCandidates())

	return FindCanonicalNode(c.HealthyCandidates())
}

// RevokeMaster revokes the leadership of the current master and returns the stopped hash.
func (c *Election) RevokeMaster() error {
	master := c.Master()
	if master == nil {
		return nil
	}

	c.log.Warn("Revoking master", "master", c.master)

	_, err := master.OpNode.StopSequencer(context.Background())
	if err != nil && !strings.Contains(err.Error(), "sequencer not running") {
		c.log.Warn("Call admin_stopSequencer when revoke master", "node", master.Name, "error", err)
	}

	// Double check if sequencer is not active
	if active, err := master.OpNode.SequencerActive(context.Background()); err == nil && !active {
		syncStatus, err := master.OpNode.SyncStatus(context.Background())
		if err == nil && syncStatus.UnsafeL2.Hash != (common.Hash{}) {
			c.master = ""
			c.prevStoppedHash = &syncStatus.UnsafeL2.Hash

			c.log.Info("Revoked master successfully", "node", master.Name, "prevStoppedHash", c.prevStoppedHash)
			return nil
		} else if err != nil {
			return fmt.Errorf("fail to call optimism_syncStatus, node: %s, error: %s", master.Name, err)
		} else {
			return fmt.Errorf("fail to call optimism_syncStatus, node: %s, error: zero hash unsafe l2", master.Name)
		}
	} else {
		return fmt.Errorf("fail to call admin_sequencerActive, node: %s, active: %s, error: %s", master.Name, active, err)
	}
}

func (c *Election) setMaster(nodeName string) error {
	node := c.Nodes[nodeName]
	if node == nil {
		return errors.New("node is not found")
	}
	if !c.IsCandidate(node.Name) {
		return errors.New("node is not a candidate")
	}
	if c.master == node.Name {
		return errors.New("node is already the master")
	}

	if err := c.startSequencer(node); err != nil {
		return err
	}

	c.log.Info("assign master", "node", nodeName)
	c.master = nodeName
	c.prevStoppedHash = nil

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
