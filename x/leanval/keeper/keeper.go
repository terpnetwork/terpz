package keeper

import (
	"os"
	"strings"

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
	pendingLNPR   *types.LNPRBlob
	memMembership [][]byte
}

func NewKeeper(store Store, v Verifier) *Keeper {
	if v == nil {
		v = DummyStwoGo{}
	}
	if store == nil {
		store = NewMemStore()
	}
	allowDummy := true
	cv := strings.ToLower(strings.TrimSpace(os.Getenv("LEAN_CONSENSUS")))
	if cv == "commonware" || cv == "cw" || cv == "simplex" {
		allowDummy = false
	}
	k := &Keeper{store: store, Verifier: v, RequireLNPR: true, AllowDummy: allowDummy}
	// Reset process RAM only. Never delete membership files: CLI NewKeeper
	// shares the container dir with the running node.
	membershipQ.mu.Lock()
	membershipQ.txs = nil
	membershipQ.mu.Unlock()
	// Fold / valset-air on PATH is the aggregate for current object roots.
	// Extra JOIN/LEAV subjects are not in that statement. Closing Dummy here
	// makes every proposer carry unverifiable extras → Process REJECT stall.
	// Do not hide that stall by proposing genesis-only when JOIN files exist.
	// Dummy on extras is lab admission, not Dummy-N as the green aggregate.
	return k
}

func (k *Keeper) SetOwnsValset(v bool) {
	k.ownsValset = v
	if k.store != nil {
		if v {
			k.live().Set(types.OwnsValsetKey(), []byte{1})
		} else {
			k.live().Set(types.OwnsValsetKey(), []byte{0})
		}
	}
}

func (k *Keeper) OwnsValset() bool {
	if k.store != nil {
		if b := k.live().Get(types.OwnsValsetKey()); len(b) > 0 {
			return b[0] != 0
		}
	}
	return k.ownsValset
}

func (k *Keeper) SetPendingUpdates(u []abci.ValidatorUpdate) {
	k.pending = append([]abci.ValidatorUpdate(nil), u...)
}

func (k *Keeper) SetEndPeriod(p uint64) { k.endPeriod = p }

func (k *Keeper) Store() Store { return k.live() }

// live returns the KV bound to the current ABCI context when one is set.
// Never cache ctx.KVStore across Prepare/Process/Finalize: a stale gaskv
// writes into a discarded cache and the new roster does not commit.
func (k *Keeper) live() Store {
	if k.hasCtx && k.sk != nil {
		return BindKV(k.sdkCtx, k.sk)
	}
	if k.store == nil {
		k.store = NewMemStore()
	}
	return k.store
}

func (k *Keeper) SetGasMeter(g storetypes.GasMeter) { k.gas = g }

// QueryBondedSet is a debug view. Membership SoT is deposit index + bitfield + EB
// (SOURCES 1A/1B). Do not treat this as the bitfield or as Comet VP wiring.
func (k *Keeper) QueryBondedSet(period uint64) []SubjectPower {
	if k.nextDepositIndex() > 0 || len(k.live().Get(types.BitfieldKey())) > 0 {
		if bits := k.DebugSubjectsFromBits(); len(bits) > 0 {
			return bits
		}
	}
	return k.BondedSet(period)
}
