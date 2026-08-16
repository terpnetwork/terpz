package leanval

import (
	"context"

	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

// DistrAppModule: Lean owns power; F1 still pays delegators from fee pool
// using BondedSet weights (not per-block VoteInfos / EB writes).
type DistrAppModule struct {
	distr.AppModule
	lean  *keeper.Keeper
	alloc keeper.TokenAllocator
}

func WrapDistribution(am distr.AppModule, lean *keeper.Keeper, alloc keeper.TokenAllocator) DistrAppModule {
	return DistrAppModule{AppModule: am, lean: lean, alloc: alloc}
}

func (am DistrAppModule) BeginBlock(ctx context.Context) error {
	if am.lean == nil || !am.lean.OwnsValset() {
		return am.AppModule.BeginBlock(ctx)
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	am.lean.BindContext(sdkCtx)
	return am.lean.AllocateDelegatorFees(sdkCtx, am.alloc)
}
