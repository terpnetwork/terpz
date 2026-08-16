package leanval

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

// StakingModule is the subset of x/staking AppModule we wrap.
type StakingModule interface {
	module.AppModule
	EndBlock(context.Context) ([]abci.ValidatorUpdate, error)
}

// WrappedStaking runs staking EndBlock (unbonding etc.) but drops ValidatorUpdates
// when Lean owns the valset so Comet is not given staking last-power.
type WrappedStaking struct {
	StakingModule
	lean *keeper.Keeper
}

func WrapStakingModule(staking StakingModule, lean *keeper.Keeper) WrappedStaking {
	return WrappedStaking{StakingModule: staking, lean: lean}
}

func (w WrappedStaking) EndBlock(ctx context.Context) ([]abci.ValidatorUpdate, error) {
	u, err := w.StakingModule.EndBlock(ctx)
	if w.lean != nil && w.lean.OwnsValset() {
		return []abci.ValidatorUpdate{}, err
	}
	return u, err
}
