package main

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	apitypes "github.com/filecoin-project/lotus/api/types"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/types"
)

const (
	mainnetGenesisTime = 1598306400
)

type API interface {
	blockstore.ChainIO
	StateNetworkVersion(context.Context, types.TipSetKey) (apitypes.NetworkVersion, error)
	ChainHead(context.Context) (*types.TipSet, error)
	StateMinerActiveSectors(context.Context, address.Address, types.TipSetKey) ([]*miner.SectorOnChainInfo, error)
	StateGetActor(context.Context, address.Address, types.TipSetKey) (*types.Actor, error)
	StateMinerInfo(context.Context, address.Address, types.TipSetKey) (api.MinerInfo, error) //perm:read
	MpoolPushMessage(context.Context, *types.Message, *api.MessageSendSpec) (*types.SignedMessage, error)
}

func TimestampToEpoch(ts int64) abi.ChainEpoch {
	return abi.ChainEpoch((ts - mainnetGenesisTime) / 30)
}
