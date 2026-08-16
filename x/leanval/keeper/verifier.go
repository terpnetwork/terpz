package keeper

import "fmt"

// Verifier is the dummy-Stwo / host verify waist.
// Production must not implement a always-true fake (DUMMY-STWO).
type Verifier interface {
	// VerifyDummy rejects on bitflip / wrong prover / oversize.
	VerifyDummy(proof, instances []byte) error
}

// ClosedVerifier never accepts. Safe default until zk-wasmvm Stwo arm exists.
type ClosedVerifier struct{}

func (ClosedVerifier) VerifyDummy(proof, instances []byte) error {
	return fmt.Errorf("leanval: no production verifier (closed)")
}

// TestVerifier is defined in verifier_test.go only.


