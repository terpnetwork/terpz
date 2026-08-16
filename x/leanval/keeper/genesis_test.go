package keeper

import (
	"encoding/json"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestInitGenesisSetsFlag(t *testing.T) {
	k := NewKeeper(nil, nil)
	k.InitGenesis(types.GenesisState{OwnsValset: true})
	if !k.OwnsValset() {
		t.Fatal("InitGenesis must set leanval_owns_valset")
	}
	gs := k.ExportGenesis()
	if !gs.OwnsValset {
		t.Fatal("ExportGenesis")
	}
}

// Exact JSON lean_terpz.rs writes into app_state.leanval.
func TestInitGenesisUnmarshalsLeanTerpzJSON(t *testing.T) {
	const ict = `{"leanval_owns_valset": true}`
	var gs types.GenesisState
	if err := json.Unmarshal([]byte(ict), &gs); err != nil {
		t.Fatal(err)
	}
	store := NewMemStore()
	k := NewKeeper(store, nil)
	k.InitGenesis(gs)
	if !k.OwnsValset() {
		t.Fatal("OwnsValset() true after InitGenesis of lean_terpz JSON")
	}
	// Persist: a new keeper on the same KV must still see the flag.
	k2 := NewKeeper(store, nil)
	if !k2.OwnsValset() {
		t.Fatal("ownsValset must persist in KV, not RAM-only")
	}
}

func TestInitGenesisSeedsSubjects(t *testing.T) {
	pub := make([]byte, 32)
	pub[0] = 0xab
	k := NewKeeper(NewMemStore(), nil)
	k.InitGenesis(types.GenesisState{
		OwnsValset: true,
		GenesisSubjects: []types.GenesisSubject{
			{PubKey: pub, Weight: 42},
		},
	})
	set := k.QueryBondedSet(0)
	if len(set) != 1 || set[0].Weight != 42 || !set[0].HasProof {
		t.Fatalf("genesis subjects must seed BondedSet: %+v", set)
	}
}
