package types

import (
	"encoding/json"
	"fmt"
)

// GenesisSubject seeds BondedSet when staking keeper is not available at InitGenesis.
type GenesisSubject struct {
	PubKey []byte `json:"pubkey"`
	Weight int64  `json:"weight"`
}

// GenesisState is parsed by InitGenesis (ICT writes these JSON keys).
// lean_terpz.rs writes: {"leanval_owns_valset": true}
// lean_waist.rs may wrap the flag under "params".
type GenesisState struct {
	OwnsValset      bool             `json:"leanval_owns_valset"`
	VerifierAddr    []byte           `json:"verifier_addr,omitempty"`
	LeanCodeID      uint64           `json:"lean_code_id,omitempty"`
	GenesisSubjects []GenesisSubject `json:"genesis_subjects,omitempty"`
	BlocksPerPeriod int64            `json:"blocks_per_period,omitempty"`
}

func DefaultGenesis() GenesisState {
	return GenesisState{OwnsValset: false}
}

// Validate is fail-closed: owns_valset with no genesis subjects cannot seed Comet VP.
func (g GenesisState) Validate() error {
	if g.OwnsValset && len(g.GenesisSubjects) == 0 {
		return fmt.Errorf("leanval: leanval_owns_valset requires genesis_subjects")
	}
	return nil
}

func (g *GenesisState) UnmarshalJSON(b []byte) error {
	type alias GenesisState
	var raw struct {
		alias
		Params *struct {
			OwnsValset bool `json:"leanval_owns_valset"`
		} `json:"params"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*g = GenesisState(raw.alias)
	if raw.Params != nil && raw.Params.OwnsValset {
		g.OwnsValset = true
	}
	return nil
}
