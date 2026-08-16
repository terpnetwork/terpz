package leanval

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

// StakingEndBlocker is x/staking AppModule.EndBlock without importing staking.
type StakingEndBlocker interface {
	EndBlock(ctx context.Context) ([]abci.ValidatorUpdate, error)
}

// WrapStakingEndBlock always runs staking EndBlock so unbonding queues mature.
// When leanval_owns_valset, returned ValidatorUpdates are dropped (app.EndBlocker
// overwrites with BondedSet). Not registered in app — staking.NewAppModule is used
// as-is; this wrap exists for callers who still compose it.
func WrapStakingEndBlock(staking StakingEndBlocker, lean *keeper.Keeper) func(context.Context) ([]abci.ValidatorUpdate, error) {
	return func(ctx context.Context) ([]abci.ValidatorUpdate, error) {
		if staking == nil {
			return nil, nil
		}
		u, err := staking.EndBlock(ctx)
		if lean != nil && lean.OwnsValset() {
			return nil, err
		}
		return u, err
	}
}
