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

// BalanceAirPublic is Lean Phase 1B public I/O (CONSTRAINT-SYSTEM).
// v1 weight is int64; later fp8 effective-balance. Tree roots may be 32 zero bytes in v1.
type BalanceAirPublic struct {
	Period          uint64
	PrevCommitment  [32]byte
	Weight          int64
	DepositTreeRoot [32]byte
	BitfieldRoot    [32]byte
	EBTreeRoot      [32]byte
	Subject         []byte
}

// BalanceAirPrivate is the Phase 1B witness (Merkle branches stub + fine balance).
type BalanceAirPrivate struct {
	ParticipationBranch []byte
	FineBalance         int64
}

// BalanceAirInstanceBytes is period(8 BE)|weight(8 BE)|subject.
// DummyStwo and a future Stwo arm must consume this layout.
func BalanceAirInstanceBytes(period uint64, weight int64, subject []byte) []byte {
	return instancesFor(period, subject, weight)
}

// BalanceAirVerifier verifies the daily-balance AIR (LEAN-4). Not DummyStwo.
type BalanceAirVerifier interface {
	VerifyBalanceAir(proof []byte, pub BalanceAirPublic, priv BalanceAirPrivate) error
}

// UnimplementedBalanceAir is the fail-closed stub until a real Stwo AIR exists.
type UnimplementedBalanceAir struct{}

func (UnimplementedBalanceAir) VerifyBalanceAir(proof []byte, pub BalanceAirPublic, priv BalanceAirPrivate) error {
	return fmt.Errorf("not implemented")
}
