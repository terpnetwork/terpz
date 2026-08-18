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

func TestCountSetBitsAndSubspacePairs(t *testing.T) {
	if n := CountSetBits([]byte{0b00000111}); n != 3 {
		t.Fatalf("CountSetBits= %d", n)
	}
	pairs := []rawKV{
		{key: BitfieldKey(), value: []byte{0x03}},
	}
	bz := encodeKVPairs(pairs)
	got, err := DecodeSubspacePairs(bz)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Value[0] != 0x03 {
		t.Fatalf("%+v", got)
	}
}
