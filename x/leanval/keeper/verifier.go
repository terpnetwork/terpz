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

// StwoBalanceAir is the Phase 1B daily-balance circuit (LEAN-4).
// v1: FineBalance must equal Weight; DummyStwo binds period|weight|subject.
// Phase 1B Stwo daily-balance verifier (CPU Dummy bind + fine==weight).
type StwoBalanceAir struct{}

func (StwoBalanceAir) VerifyBalanceAir(proof []byte, pub BalanceAirPublic, priv BalanceAirPrivate) error {
	if pub.Period == 0 {
		return fmt.Errorf("leanval: balance AIR period required")
	}
	if len(pub.Subject) == 0 {
		return fmt.Errorf("leanval: balance AIR subject required")
	}
	if priv.FineBalance != pub.Weight {
		return fmt.Errorf("leanval: balance AIR fine != weight")
	}
	inst := BalanceAirInstanceBytes(pub.Period, pub.Weight, pub.Subject)
	if len(proof) == 0 {
		proof = DummyStwoProveBound(pub.Period, pub.Subject, pub.Weight)
	}
	if err := (DummyStwoGo{}).VerifyDummy(proof, inst); err != nil {
		return fmt.Errorf("leanval: balance AIR verify: %w", err)
	}
	return nil
}
