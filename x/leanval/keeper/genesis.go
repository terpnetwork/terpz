package keeper

import "github.com/terpnetwork/terp-core/v6/x/leanval/types"

func (k *Keeper) InitGenesis(gs types.GenesisState) {
	k.SetOwnsValset(gs.OwnsValset)
}

func (k *Keeper) ExportGenesis() types.GenesisState {
	return types.GenesisState{OwnsValset: k.OwnsValset()}
}
