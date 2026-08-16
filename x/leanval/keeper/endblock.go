package keeper

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
)

// EndBlock only flushes a pending set if a caller queued one.
// Production valset is app.EndBlocker → ValidatorUpdates (stomp). Do not invent
// a second BondedSet story here.
func (k *Keeper) EndBlock(_ context.Context) ([]abci.ValidatorUpdate, error) {
	if !k.OwnsValset() {
		return nil, nil
	}
	if len(k.pending) > 0 {
		out := k.pending
		k.pending = nil
		return out, nil
	}
	return nil, nil
}
