package keeper

import (
	"crypto/sha256"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Bitfield + deposit index + EB tree are membership SoT (SOURCES 1A/1B).
// BondedSet is a debug view only.

func (k *Keeper) nextDepositIndex() uint32 {
	return types.GetU32(k.store.Get(types.NextDepositIndexKey()))
}

func (k *Keeper) setNextDepositIndex(n uint32) {
	k.store.Set(types.NextDepositIndexKey(), types.PutU32(n))
}

func (k *Keeper) Bitfield() []byte {
	b := k.store.Get(types.BitfieldKey())
	if len(b) == 0 {
		return []byte{0}
	}
	return append([]byte(nil), b...)
}

func bitGet(bf []byte, idx uint32) bool {
	i := int(idx / 8)
	if i >= len(bf) {
		return false
	}
	return bf[i]&(1<<uint(idx%8)) != 0
}

func bitSet(bf []byte, idx uint32, on bool) []byte {
	i := int(idx / 8)
	need := i + 1
	if len(bf) < need {
		n := make([]byte, need)
		copy(n, bf)
		bf = n
	} else {
		bf = append([]byte(nil), bf...)
	}
	mask := byte(1 << uint(idx%8))
	if on {
		bf[i] |= mask
	} else {
		bf[i] &^= mask
	}
	return bf
}

func (k *Keeper) setBitfield(b []byte) {
	if len(b) == 0 {
		b = []byte{0}
	}
	k.store.Set(types.BitfieldKey(), append([]byte(nil), b...))
}

func (k *Keeper) BitSet(idx uint32) {
	k.setBitfield(bitSet(k.Bitfield(), idx, true))
	k.syncObjectRoots()
}

func (k *Keeper) BitClear(idx uint32) {
	k.setBitfield(bitSet(k.Bitfield(), idx, false))
	k.syncObjectRoots()
}

func (k *Keeper) BitIsSet(idx uint32) bool {
	return bitGet(k.Bitfield(), idx)
}

func (k *Keeper) ORMergeBitfield(other []byte) {
	cur := k.Bitfield()
	n := len(cur)
	if len(other) > n {
		n = len(other)
	}
	out := make([]byte, n)
	copy(out, cur)
	for i := 0; i < len(other); i++ {
		out[i] |= other[i]
	}
	k.setBitfield(out)
	k.syncObjectRoots()
}

func (k *Keeper) depositIndexOf(subject []byte) (uint32, bool) {
	v := k.store.Get(types.DepositIndexKey(subject))
	if len(v) < 5 {
		return 0, false
	}
	return types.GetU32(v[len(v)-4:]), true
}

func (k *Keeper) AllocateIndex(subject []byte) uint32 {
	if idx, ok := k.depositIndexOf(subject); ok {
		return idx
	}
	n := k.nextDepositIndex()
	k.store.Set(types.DepositIndexKey(subject), types.DepositIndexBytes(n))
	k.setNextDepositIndex(n + 1)
	k.syncObjectRoots()
	return n
}

func ebFromWeight(w int64) byte {
	if w <= 0 {
		return 0
	}
	if w > 255 {
		return 255
	}
	return byte(w)
}

func (k *Keeper) SetEB(idx uint32, eb byte) {
	k.store.Set(types.EBKey(idx), []byte{eb})
	k.syncObjectRoots()
}

func (k *Keeper) EBOf(idx uint32) byte {
	v := k.store.Get(types.EBKey(idx))
	if len(v) == 0 {
		return 0
	}
	return v[0]
}

func (k *Keeper) admitMember(subject []byte, weight int64) uint32 {
	idx := k.AllocateIndex(subject)
	if weight > 0 {
		// Do not overwrite an explicit EB tree byte (vp-from-bits tests).
		if len(k.store.Get(types.EBKey(idx))) == 0 {
			k.SetEB(idx, ebFromWeight(weight))
		}
		k.BitSet(idx)
	}
	return idx
}

func sha256Root(b []byte) [32]byte {
	return sha256.Sum256(b)
}

func (k *Keeper) BitfieldRoot() [32]byte {
	// Commitment to bitfield bytes only — not SHA256 of another map's concat.
	return sha256Root(k.Bitfield())
}

func (k *Keeper) DepositTreeRoot() [32]byte {
	return merkleRoot(k.depositLeaves())
}

func (k *Keeper) EBTreeRoot() [32]byte {
	return merkleRoot(k.ebLeaves())
}

func (k *Keeper) syncObjectRoots() {
	var roots [96]byte
	bf := k.BitfieldRoot()
	dep := k.DepositTreeRoot()
	eb := k.EBTreeRoot()
	copy(roots[0:32], bf[:])
	copy(roots[32:64], dep[:])
	copy(roots[64:96], eb[:])
	k.SetLastObjectRoots(roots[:])
}

func (k *Keeper) subjectByIndex(want uint32) []byte {
	var found []byte
	k.store.IteratePrefix([]byte{types.DepositIndexPrefix}, func(key, value []byte) bool {
		if len(value) < 5 {
			return true
		}
		if types.GetU32(value[len(value)-4:]) == want {
			found = append([]byte(nil), key[1:]...)
			return false
		}
		return true
	})
	return found
}

// DebugSubjectsFromBits rebuilds BondedSet-shaped rows from set bits + EB.
func (k *Keeper) DebugSubjectsFromBits() []SubjectPower {
	n := k.nextDepositIndex()
	out := make([]SubjectPower, 0, n)
	for i := uint32(0); i < n; i++ {
		if !k.BitIsSet(i) {
			continue
		}
		subj := k.subjectByIndex(i)
		if len(subj) == 0 {
			subj = types.DepositIndexBytes(i)
		}
		eb := k.EBOf(i)
		w := int64(eb)
		out = append(out, SubjectPower{Subject: subj, Weight: w, HasProof: eb > 0})
	}
	return out
}
