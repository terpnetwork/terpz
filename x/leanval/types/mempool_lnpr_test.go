package types

import "testing"

func TestRejectMempoolLNPR(t *testing.T) {
	if err := RejectMempoolLNPR(append(append([]byte{}, PrefixLNPR...), 0x01)); err != ErrMempoolLNPR {
		t.Fatalf("%v", err)
	}
	if err := RejectMempoolLNPR([]byte{0x0a, 0x0b}); err != nil {
		t.Fatal(err)
	}
}
