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

// WrapStakingEndBlock skips staking EndBlock when leanval_owns_valset.
func WrapStakingEndBlock(staking StakingEndBlocker, lean *keeper.Keeper) func(context.Context) ([]abci.ValidatorUpdate, error) {
	return func(ctx context.Context) ([]abci.ValidatorUpdate, error) {
		if lean != nil && lean.OwnsValset() {
			return nil, nil
		}
		if staking == nil {
			return nil, nil
		}
		return staking.EndBlock(ctx)
	}
}
