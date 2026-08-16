package types

import "testing"

func TestDecodeBondedSubspace(t *testing.T) {
	subj := []byte{0x11, 0x22}
	key := BondedKey(7, subj)
	val := make([]byte, 9)
	val[0] = 1
	copy(val[1:], PutI64(42))
	bz := encodeKVPairs([]rawKV{{key: key, value: val}})
	rows, err := DecodeBondedSubspace(7, bz)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Weight != 42 || !rows[0].HasProof {
		t.Fatalf("%+v", rows)
	}
}
