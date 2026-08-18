package keeper

import (
	"crypto/sha256"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Append-only binary Merkle trees. Leaf position == deposit index.
// Roots are not SHA256(concat(store KV)).

func hashLeaf(data []byte) [32]byte {
	return sha256.Sum256(data)
}

func hashNode(left, right [32]byte) [32]byte {
	var buf [64]byte
	copy(buf[:32], left[:])
	copy(buf[32:], right[:])
	return sha256.Sum256(buf[:])
}

// merkleRoot folds leaves bottom-up; odd last leaf is duplicated.
func merkleRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 0 {
		return hashLeaf(nil)
	}
	layer := append([][32]byte(nil), leaves...)
	for len(layer) > 1 {
		if len(layer)%2 == 1 {
			layer = append(layer, layer[len(layer)-1])
		}
		next := make([][32]byte, 0, len(layer)/2)
		for i := 0; i < len(layer); i += 2 {
			next = append(next, hashNode(layer[i], layer[i+1]))
		}
		layer = next
	}
	return layer[0]
}

func depositLeaf(subject []byte, idx uint32) [32]byte {
	buf := make([]byte, 0, len(subject)+4)
	buf = append(buf, subject...)
	buf = append(buf, types.PutU32(idx)...)
	return hashLeaf(buf)
}

func ebLeaf(eb byte) [32]byte {
	return hashLeaf([]byte{eb})
}

func (k *Keeper) depositLeaves() [][32]byte {
	n := k.nextDepositIndex()
	leaves := make([][32]byte, n)
	for i := uint32(0); i < n; i++ {
		subj := k.subjectByIndex(i)
		leaves[i] = depositLeaf(subj, i)
	}
	return leaves
}

func (k *Keeper) ebLeaves() [][32]byte {
	n := k.nextDepositIndex()
	leaves := make([][32]byte, n)
	for i := uint32(0); i < n; i++ {
		leaves[i] = ebLeaf(k.EBOf(i))
	}
	return leaves
}
