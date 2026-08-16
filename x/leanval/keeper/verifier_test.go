package keeper

import "fmt"

// TestVerifier is for tests only.
type TestVerifier struct {
	Accept []byte
}

func (v TestVerifier) VerifyDummy(proof, instances []byte) error {
	if len(proof) == 0 {
		return fmt.Errorf("empty proof")
	}
	if string(proof) != string(v.Accept) {
		return fmt.Errorf("verify dummy failed")
	}
	return nil
}
