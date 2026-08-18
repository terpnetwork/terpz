package keeper

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// SHA256(concat KV) must not be accepted as a tree root (SOURCES 1A/1B).

func concatDepositKV(k *Keeper) []byte {
	var acc []byte
	k.store.IteratePrefix([]byte{types.DepositIndexPrefix}, func(key, value []byte) bool {
		acc = append(acc, key[1:]...)
		acc = append(acc, value...)
		return true
	})
	return acc
}

func concatEBKV(k *Keeper) []byte {
	var acc []byte
	k.store.IteratePrefix([]byte{types.EBPrefix}, func(key, value []byte) bool {
		acc = append(acc, key[1:]...)
		acc = append(acc, value...)
		return true
	})
	return acc
}

func TestMerkleRoots_RejectSHA256OfConcatKV(t *testing.T) {
	a, b := testPub(0x01), testPub(0x02)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.InitGenesis(types.GenesisState{
		OwnsValset: true,
		GenesisSubjects: []types.GenesisSubject{
			{PubKey: a, Weight: 10},
			{PubKey: b, Weight: 20},
		},
	})

	depConcat := sha256.Sum256(concatDepositKV(k))
	gotDep := k.DepositTreeRoot()
	if bytes.Equal(gotDep[:], depConcat[:]) {
		t.Fatal("DepositTreeRoot must be a Merkle root, not SHA256(concat deposit-index KV)")
	}
	if k.nextDepositIndex() < 2 {
		t.Fatal("need >=2 leaves so Merkle parent != single-leaf hash")
	}
	wantDep := merkleRoot(k.depositLeaves())
	if gotDep != wantDep {
		t.Fatalf("DepositTreeRoot want merkle %x got %x", wantDep, gotDep)
	}

	ebConcat := sha256.Sum256(concatEBKV(k))
	gotEB := k.EBTreeRoot()
	if bytes.Equal(gotEB[:], ebConcat[:]) {
		t.Fatal("EBTreeRoot must be a Merkle root, not SHA256(concat EB-map KV)")
	}
	wantEB := merkleRoot(k.ebLeaves())
	if gotEB != wantEB {
		t.Fatalf("EBTreeRoot want merkle %x got %x", wantEB, gotEB)
	}

	bf := k.Bitfield()
	bfOnly := sha256.Sum256(bf)
	gotBF := k.BitfieldRoot()
	if gotBF != bfOnly {
		t.Fatalf("BitfieldRoot must commit to bitfield bytes, want %x got %x", bfOnly, gotBF)
	}
	costume := append(append([]byte{}, concatDepositKV(k)...), concatEBKV(k)...)
	costumeH := sha256.Sum256(costume)
	if bytes.Equal(gotBF[:], costumeH[:]) {
		t.Fatal("BitfieldRoot must not be SHA256 of concatenated sibling maps")
	}
}

func TestMerkleRoots_IndexIsLeafPosition(t *testing.T) {
	a := testPub(0xaa)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	idx := k.AllocateIndex(a)
	if idx != 0 {
		t.Fatalf("first leaf index 0, got %d", idx)
	}
	leaf := depositLeaf(a, 0)
	root := k.DepositTreeRoot()
	if root != leaf {
		t.Fatalf("single-leaf deposit tree root must equal leaf; root=%x leaf=%x", root, leaf)
	}
}
