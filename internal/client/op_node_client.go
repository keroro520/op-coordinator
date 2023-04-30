package client

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
	"time"
)

type OpNodeClient struct {
	Name         string
	Url          string
	RollupClient RollupClient
	p2pClient    p2p.Client
	TryTimes     uint8
	TrySleepTime uint64
}

func (this *OpNodeClient) Block(ctx context.Context, peerId string) {
	this.p2pClient.BlockPeer(ctx, peer.ID(peerId))
	this.p2pClient.DisconnectPeer(ctx, peer.ID(peerId))
	//todo self test
}

func (this *OpNodeClient) startSequencer(ctx context.Context, peerId string) {
	//todo self test
}

func (this *OpNodeClient) IsMaster(ctx context.Context) (bool, error) {
	//todo self test
	var i uint8
	var err error
	for i = 0; i <= this.TryTimes; i++ {
		isMater, err := this.RollupClient.IsMaster(ctx)
		if err != nil {
			return *isMater, nil
		}
		zap.S().Warnf("Request(%v %v) try times is  %v, IsMaster ", this.Name, this.Url, i, err)
		if i < this.TryTimes && this.TrySleepTime > 0 {
			time.Sleep(time.Microsecond * time.Duration(this.TrySleepTime))
		}
	}
	return false, err

}
func (this *OpNodeClient) Unsafe(ctx context.Context) (uint64, error) {
	var i uint8
	var err error
	for i = 0; i <= this.TryTimes; i++ {
		syncStatus, err := this.RollupClient.SyncStatus(ctx)
		if err != nil && syncStatus != nil {
			return syncStatus.UnsafeL2.Number, nil
		}
		zap.S().Warnf("Request(%v %v) try times is %v, Unsafe ", this.Name, this.Url, i, err)
		if i < this.TryTimes && this.TrySleepTime > 0 {
			time.Sleep(time.Microsecond * time.Duration(this.TrySleepTime))
		}

	}
	return 0, err
	//todo self test
}
