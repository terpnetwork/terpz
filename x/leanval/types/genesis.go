package types

// GenesisState is parsed by InitGenesis (ICT writes these JSON keys).
type GenesisState struct {
	OwnsValset   bool   `json:"leanval_owns_valset"`
	VerifierAddr []byte `json:"verifier_addr,omitempty"`
	LeanCodeID   uint64 `json:"lean_code_id,omitempty"`
}

func DefaultGenesis() GenesisState {
	return GenesisState{OwnsValset: false}
}
