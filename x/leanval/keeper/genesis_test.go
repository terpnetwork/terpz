package keeper

import (
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
