package types

import (
	"bytes"
	"testing"
)

func TestJoinLeaveRoundTrip(t *testing.T) {
	subj := bytes.Repeat([]byte{0xab}, 32)
	j := EncodeJoin(JoinBlob{Period: 3, Subject: subj, Weight: 12})
	got, ok := DecodeJoin(j)
	if !ok || got.Period != 3 || got.Weight != 12 || !bytes.Equal(got.Subject, subj) {
		t.Fatalf("join decode: %+v ok=%v", got, ok)
	}
	if !IsMembershipTx(j) {
		t.Fatal("join should be membership")
	}
	l := EncodeLeave(LeaveBlob{Period: 3, Subject: subj})
	lg, ok := DecodeLeave(l)
	if !ok || lg.Period != 3 || !bytes.Equal(lg.Subject, subj) {
		t.Fatalf("leave decode: %+v ok=%v", lg, ok)
	}
	if RejectMempoolLNPR(j) != nil || RejectMempoolLNPR(l) != nil {
		t.Fatal("JOIN/LEAV must not be inject-class rejected")
	}
}
