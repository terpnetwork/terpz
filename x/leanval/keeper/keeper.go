package keeper

import (
	abci "github.com/cometbft/cometbft/abci/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

// Keeper owns bonded_set(P) only. After cutover this is the sole ValidatorUpdate source.
type Keeper struct {
	store Store
	// Verifier is DummyStwoGo unless tests inject TestVerifier / ClosedVerifier.
	Verifier    Verifier
	RequireLNPR bool
	ownsValset  bool
	pending     []abci.ValidatorUpdate
	endPeriod   uint64
	sk          storetypes.StoreKey
}

func NewKeeper(store Store, v Verifier) *Keeper {
	if v == nil {
		v = DummyStwoGo{}
	}
	if store == nil {
		store = NewMemStore()
	}
	return &Keeper{store: store, Verifier: v, RequireLNPR: true}
}

func (k *Keeper) SetOwnsValset(v bool) { k.ownsValset = v }
func (k *Keeper) OwnsValset() bool     { return k.ownsValset }

func (k *Keeper) SetPendingUpdates(u []abci.ValidatorUpdate) {
	k.pending = append([]abci.ValidatorUpdate(nil), u...)
}

func (k *Keeper) SetEndPeriod(p uint64) { k.endPeriod = p }

func (k *Keeper) Store() Store { return k.store }

func (k *Keeper) QueryBondedSet(period uint64) []SubjectPower {
	return k.BondedSet(period)
}
