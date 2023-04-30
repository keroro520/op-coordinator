package client

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

type OpNodeClient struct {
	url            string
	RollupClient   RollupClient
	p2pClient      p2p.Client
	RequestRetries uint8
}

func (this *OpNodeClient) Block(ctx context.Context, peerId string) {
	this.p2pClient.BlockPeer(ctx, peer.ID(peerId))
	this.p2pClient.DisconnectPeer(ctx, peer.ID(peerId))
	//todo self test
}

func (this *OpNodeClient) IsMaster(ctx context.Context) (bool, error) {
	//todo self test
	var i uint8
	var err error
	for i = 0; i <= this.RequestRetries; i++ {
		isMater, err := this.RollupClient.IsMaster(ctx)
		if err != nil {
			return *isMater, nil
		}
	}
	return false, err

}
func (this *OpNodeClient) Unsafe(ctx context.Context) (uint64, error) {
	var i uint8
	var err error
	for i = 0; i <= this.RequestRetries; i++ {
		syncStatus, err := this.RollupClient.SyncStatus(ctx)
		if err != nil && syncStatus != nil {
			return syncStatus.UnsafeL2.Number, nil
		}
	}
	return 0, err
	//todo self test
}
