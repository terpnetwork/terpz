package keeper

import (
	abci "github.com/cometbft/cometbft/abci/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Keeper owns bonded_set(P) only. After cutover this is the sole ValidatorUpdate source.
type Keeper struct {
	store Store
	// Verifier is DummyStwoGo unless tests inject TestVerifier / ClosedVerifier.
	Verifier      Verifier
	RequireLNPR   bool
	AllowDummy    bool
	ownsValset    bool
	pending       []abci.ValidatorUpdate
	endPeriod     uint64
	sk            storetypes.StoreKey
	gas           storetypes.GasMeter
	wasmSudo      WasmSudoClient
	verifierAcc   []byte
	leanZkID      uint64
	sdkCtx        sdk.Context
	hasCtx        bool
	memMembership [][]byte
}

func NewKeeper(store Store, v Verifier) *Keeper {
	if v == nil {
		v = DummyStwoGo{}
	}
	if store == nil {
		store = NewMemStore()
	}
	k := &Keeper{store: store, Verifier: v, RequireLNPR: true, AllowDummy: true}
	if leanValsetAirBin() != "" {
		k.AllowDummy = false
	}
	return k
}

func (k *Keeper) SetOwnsValset(v bool) {
	k.ownsValset = v
	if k.store != nil {
		if v {
			k.store.Set(types.OwnsValsetKey(), []byte{1})
		} else {
			k.store.Set(types.OwnsValsetKey(), []byte{0})
		}
	}
}

func (k *Keeper) OwnsValset() bool {
	if k.store != nil {
		if b := k.store.Get(types.OwnsValsetKey()); len(b) > 0 {
			return b[0] != 0
		}
	}
	return k.ownsValset
}

func (k *Keeper) SetPendingUpdates(u []abci.ValidatorUpdate) {
	k.pending = append([]abci.ValidatorUpdate(nil), u...)
}

func (k *Keeper) SetEndPeriod(p uint64) { k.endPeriod = p }

func (k *Keeper) Store() Store { return k.store }

func (k *Keeper) SetGasMeter(g storetypes.GasMeter) { k.gas = g }

// QueryBondedSet is the keeper query (CLI: terpz query leanval bonded-set [period]).
func (k *Keeper) QueryBondedSet(period uint64) []SubjectPower {
	return k.BondedSet(period)
}
