package main

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	apitypes "github.com/filecoin-project/lotus/api/types"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/types"
	cliutil "github.com/filecoin-project/lotus/cli/util"
	"github.com/filecoin-project/lotus/node/repo"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
	"time"
)

var (
	genesisTime time.Time
)

type API interface {
	blockstore.ChainIO
	ChainGetGenesis(context.Context) (*types.TipSet, error) //perm:read
	StateNetworkVersion(context.Context, types.TipSetKey) (apitypes.NetworkVersion, error)
	ChainHead(context.Context) (*types.TipSet, error)
	StateMinerActiveSectors(context.Context, address.Address, types.TipSetKey) ([]*miner.SectorOnChainInfo, error)
	StateGetActor(context.Context, address.Address, types.TipSetKey) (*types.Actor, error)
	StateMinerInfo(context.Context, address.Address, types.TipSetKey) (api.MinerInfo, error) //perm:read
	MpoolPushMessage(context.Context, *types.Message, *api.MessageSendSpec) (*types.SignedMessage, error)
	StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*api.MsgLookup, error)
}

func TimestampToEpoch(ts time.Time) abi.ChainEpoch {
	sinceGenesis := ts.Sub(genesisTime)
	expectedHeight := int64(sinceGenesis.Seconds()) / int64(build.BlockDelaySecs)
	return abi.ChainEpoch(expectedHeight)
}

func SetupGenesisTime(ts time.Time) {
	genesisTime = ts
}

func GetFullNodeAPI(ctx *cli.Context) (api.FullNode, jsonrpc.ClientCloser, error) {
	addr, headers, err := cliutil.GetRawAPI(ctx, repo.FullNode, "v1")
	if err != nil {
		return nil, nil, err
	}

	if cliutil.IsVeryVerbose {
		_, _ = fmt.Fprintln(ctx.App.Writer, "using full node API v1 endpoint:", addr)
	}
	return client.NewFullNodeRPCV1(ctx.Context, addr, headers, jsonrpc.WithTimeout(15*time.Minute))
}
