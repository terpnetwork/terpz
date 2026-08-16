package keeper

import (
	"encoding/json"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestInitGenesisSetsFlag(t *testing.T) {
	pub := make([]byte, 32)
	pub[0] = 1
	k := NewKeeper(NewMemStore(), nil)
	k.InitGenesis(types.GenesisState{
		OwnsValset:      true,
		GenesisSubjects: []types.GenesisSubject{{PubKey: pub, Weight: 1}},
	})
	if !k.OwnsValset() {
		t.Fatal("InitGenesis must set leanval_owns_valset")
	}
	gs := k.ExportGenesis()
	if !gs.OwnsValset {
		t.Fatal("ExportGenesis")
	}
}

func TestInitGenesisOwnsEmptyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("owns without subjects must panic")
		}
	}()
	k := NewKeeper(NewMemStore(), nil)
	k.InitGenesis(types.GenesisState{OwnsValset: true})
}

// ICT must now send genesis_subjects when owns is true.
func TestInitGenesisUnmarshalsLeanTerpzJSON(t *testing.T) {
	pub := make([]byte, 32)
	pub[0] = 0xcd
	raw, err := json.Marshal(types.GenesisState{
		OwnsValset:      true,
		GenesisSubjects: []types.GenesisSubject{{PubKey: pub, Weight: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var gs types.GenesisState
	if err := json.Unmarshal(raw, &gs); err != nil {
		t.Fatal(err)
	}
	store := NewMemStore()
	k := NewKeeper(store, nil)
	k.InitGenesis(gs)
	if !k.OwnsValset() {
		t.Fatal("OwnsValset() true after InitGenesis")
	}
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

func TestBondedSetOrCarryCopiesPeriod(t *testing.T) {
	pub := make([]byte, 32)
	pub[0] = 0x11
	k := NewKeeper(NewMemStore(), nil)
	k.AcceptProof(0, pub, 9)
	if got := k.BondedSet(1); len(got) != 0 {
		t.Fatalf("period 1 must start empty: %+v", got)
	}
	carried := k.BondedSetOrCarry(1)
	if len(carried) != 1 || carried[0].Weight != 9 || !carried[0].HasProof {
		t.Fatalf("carry: %+v", carried)
	}
	if got := k.BondedSet(1); len(got) != 1 || got[0].Weight != 9 {
		t.Fatalf("carry must persist into period 1: %+v", got)
	}
}
