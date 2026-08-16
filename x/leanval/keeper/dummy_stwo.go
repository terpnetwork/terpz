package keeper

import (
	"encoding/binary"
	"fmt"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Byte-identical to crates/lean-stwo-dummy DummyStwo::prove/verify (DSTW | 2 | 5 | a | b | c).
// Not pinned stwo. Named Dummy so we do not launder this as S-two.

const (
	dummyMagic          = "DSTW"
	dummyProverID       = 2
	dummyCurveID        = 5
	dummyProofLen       = 18
	m31P         uint32 = (1 << 31) - 1
	stwoDummyGas uint64 = 150_000
)

// DummyStwoGo rejects bitflips. Default production Verifier for the dummy gate.
type DummyStwoGo struct{}

func DummyStwoProve(a, b uint32) []byte {
	c := dummyM31Hash(a, b)
	out := make([]byte, dummyProofLen)
	copy(out[0:4], dummyMagic)
	out[4] = dummyProverID
	out[5] = dummyCurveID
	binary.LittleEndian.PutUint32(out[6:10], a)
	binary.LittleEndian.PutUint32(out[10:14], b)
	binary.LittleEndian.PutUint32(out[14:18], c)
	return out
}

func (DummyStwoGo) VerifyDummy(proof, _ []byte) error {
	// Gas belongs on sdk.Context; testers: STWO_DUMMY_VERIFY_GAS = 150000 before this call.
	_ = stwoDummyGas
	if len(proof) > types.MaxProofBytes {
		return fmt.Errorf("leanval: proof too large")
	}
	if len(proof) < dummyProofLen {
		return fmt.Errorf("leanval: truncated DummyStwo")
	}
	if string(proof[0:4]) != dummyMagic {
		return fmt.Errorf("leanval: bad DummyStwo magic")
	}
	if proof[4] != dummyProverID {
		return fmt.Errorf("leanval: wrong prover_id %d", proof[4])
	}
	if proof[5] != dummyCurveID {
		return fmt.Errorf("leanval: wrong curve_id %d", proof[5])
	}
	a := binary.LittleEndian.Uint32(proof[6:10])
	b := binary.LittleEndian.Uint32(proof[10:14])
	c := binary.LittleEndian.Uint32(proof[14:18])
	if a >= m31P || b >= m31P || c >= m31P {
		return fmt.Errorf("leanval: not in M31")
	}
	if dummyM31Hash(a, b) != c {
		return fmt.Errorf("leanval: DummyStwo statement false")
	}
	return nil
}

func dummyM31Hash(a, b uint32) uint32 {
	const alpha, beta, gamma uint64 = 3, 5, 7
	p := uint64(m31P)
	return uint32((alpha*uint64(a) + beta*uint64(b) + gamma) % p)
}
