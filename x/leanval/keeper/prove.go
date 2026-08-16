package keeper

import "github.com/terpnetwork/terp-core/v6/x/leanval/types"

func (k *Keeper) proveSubject(period, index uint64, subject []byte, weight int64, roots []byte) []byte {
	eb := uint8(0)
	if weight > 0 {
		if weight > 255 {
			eb = 255
		} else {
			eb = uint8(weight)
		}
	}
	if proof, err := proveValsetStwo(period, index, eb, subject); err == nil && len(proof) >= 4 {
		return proof
	}
	if k.AllowDummy {
		return DummyStwoProveBoundRoots(period, subject, weight, roots)
	}
	return nil
}

func (k *Keeper) verifySubjectProof(period, index uint64, s types.SubjectProof, roots []byte) error {
	if len(s.Proof) >= 4 && string(s.Proof[:4]) == "STWO" {
		eb := uint8(0)
		if s.Weight > 0 && s.Weight <= 255 {
			eb = uint8(s.Weight)
		} else if s.Weight > 255 {
			eb = 255
		}
		return verifyValsetStwo(s.Proof, period, index, eb, s.Subject)
	}
	if !k.AllowDummy {
		return errProof("STWO proof required (Dummy DSTW disabled)")
	}
	inst := instancesForRoots(period, s.Subject, s.Weight, roots)
	return k.verifyProof(s.Proof, inst)
}
