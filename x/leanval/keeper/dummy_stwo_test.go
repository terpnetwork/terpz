package keeper

import "testing"

func TestDummyStwoValidAndBitflip(t *testing.T) {
	p := DummyStwoProve(11, 22)
	if err := (DummyStwoGo{}).VerifyDummy(p, nil); err != nil {
		t.Fatal(err)
	}
	p[14] ^= 1
	if err := (DummyStwoGo{}).VerifyDummy(p, nil); err == nil {
		t.Fatal("bitflip must fail")
	}
}

func TestDummyStwoWrongProverID(t *testing.T) {
	p := DummyStwoProve(1, 2)
	p[4] = 0
	if err := (DummyStwoGo{}).VerifyDummy(p, nil); err == nil {
		t.Fatal("want fail closed")
	}
}
