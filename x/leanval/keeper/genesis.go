package keeper

import "github.com/terpnetwork/terp-core/v6/x/leanval/types"

func (k *Keeper) InitGenesis(gs types.GenesisState) {
	if err := gs.Validate(); err != nil {
		panic(err)
	}
	n := gs.BlocksPerPeriod
	if n <= 0 {
		n = types.BlocksPerPeriod
	}
	k.live().Set(types.PeriodParamKey(), types.PutI64(n))
	types.SetBlocksPerPeriod(n)
	k.SetOwnsValset(gs.OwnsValset)
	if len(gs.VerifierAddr) > 0 {
		k.SetModuleVerifier(gs.VerifierAddr, gs.LeanCodeID)
	}
	// Seed period 0 BondedSet from genesis_subjects (pubkey + weight).
	for _, s := range gs.GenesisSubjects {
		if len(s.PubKey) == 0 {
			continue
		}
		if s.Weight > 0 {
			k.AcceptProof(0, s.PubKey, s.Weight)
		} else {
			k.PutSubject(0, s.PubKey, 0)
		}
	}
	k.syncObjectRoots()
}

func (k *Keeper) ExportGenesis() types.GenesisState {
	gs := types.GenesisState{OwnsValset: k.OwnsValset(), BlocksPerPeriod: types.BlocksPerPeriodLive()}
	for _, s := range k.DebugSubjectsFromBits() {
		gs.GenesisSubjects = append(gs.GenesisSubjects, types.GenesisSubject{
			PubKey: append([]byte(nil), s.Subject...),
			Weight: s.Weight,
		})
	}
	return gs
}
