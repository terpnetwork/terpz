package keeper

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
)

// EndBlock emits ValidatorUpdates only when leanval_owns_valset is set.
// Flag off: nil (staking remains the sole valset source).
func (k *Keeper) EndBlock(_ context.Context) ([]abci.ValidatorUpdate, error) {
	if !k.OwnsValset() {
		return nil, nil
	}
	if len(k.pending) > 0 {
		out := k.pending
		k.pending = nil
		return out, nil
	}
	return k.ValidatorUpdates(k.endPeriod), nil
}
