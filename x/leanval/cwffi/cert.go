package cwffi

import (
	"encoding/binary"
	"fmt"
)

const lcertMagic = "LCERT"
const lcertVersion = 1

// CertSigner is one attributable Finalization vote.
type CertSigner struct {
	Index     uint32
	Signature []byte
}

// LeanCert is the LCERT wire exported from simplex Finalization.
type LeanCert struct {
	Epoch   uint64
	View    uint64
	Digest  []byte
	Signers []CertSigner
	Raw     []byte // Commonware-encoded Finalization (verify + Floor)
}

func ParseLeanCert(b []byte) (LeanCert, error) {
	var z LeanCert
	if len(b) < 5+1+8+8+32+4 {
		return z, fmt.Errorf("lean-cw: truncated certificate")
	}
	if string(b[:5]) != lcertMagic {
		// Treat as raw Finalization (Floor file).
		z.Raw = append([]byte(nil), b...)
		return z, nil
	}
	if b[5] != lcertVersion {
		return z, fmt.Errorf("lean-cw: bad cert version %d", b[5])
	}
	i := 6
	z.Epoch = binary.BigEndian.Uint64(b[i : i+8])
	i += 8
	z.View = binary.BigEndian.Uint64(b[i : i+8])
	i += 8
	z.Digest = append([]byte(nil), b[i:i+32]...)
	i += 32
	n := int(binary.BigEndian.Uint32(b[i : i+4]))
	i += 4
	if n < 0 || n > 4096 {
		return z, fmt.Errorf("lean-cw: absurd signer count %d", n)
	}
	need := i + n*(4+64) + 4
	if len(b) < need {
		return z, fmt.Errorf("lean-cw: truncated signers")
	}
	z.Signers = make([]CertSigner, n)
	for k := 0; k < n; k++ {
		idx := binary.BigEndian.Uint32(b[i : i+4])
		i += 4
		sig := append([]byte(nil), b[i:i+64]...)
		i += 64
		z.Signers[k] = CertSigner{Index: idx, Signature: sig}
	}
	rawLen := int(binary.BigEndian.Uint32(b[i : i+4]))
	i += 4
	if rawLen < 0 || i+rawLen > len(b) {
		return z, fmt.Errorf("lean-cw: truncated raw finalization")
	}
	z.Raw = append([]byte(nil), b[i:i+rawLen]...)
	return z, nil
}

func (c LeanCert) HasIndex(idx uint32) bool {
	for _, s := range c.Signers {
		if s.Index == idx {
			return true
		}
	}
	return false
}

func (c LeanCert) HasPubkey(pks []byte, pk []byte) bool {
	if len(pk) != 32 || len(pks)%32 != 0 {
		return false
	}
	for _, s := range c.Signers {
		off := int(s.Index) * 32
		if off+32 > len(pks) {
			continue
		}
		if string(pks[off:off+32]) == string(pk) {
			return true
		}
	}
	return false
}

func signedWeight(c LeanCert, weights []uint64) uint64 {
	var w uint64
	for _, s := range c.Signers {
		if int(s.Index) < len(weights) {
			w += weights[s.Index]
		}
	}
	return w
}

func totalWeight(weights []uint64) uint64 {
	var t uint64
	for _, w := range weights {
		t += w
	}
	return t
}

// MeetsWeight is strict >2/3 of BondedSet EB.
func MeetsWeight(signed, total uint64) bool {
	return total > 0 && signed*3 > total*2
}

func containsDSTW(b []byte) bool {
	for i := 0; i+4 <= len(b); i++ {
		if string(b[i:i+4]) == "DSTW" {
			return true
		}
	}
	return false
}
