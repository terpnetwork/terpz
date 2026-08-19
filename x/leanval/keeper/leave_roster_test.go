package keeper

import (
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestLeaveAfterJoinDoesNotRejoin(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.ClearPendingMembership()
	g0 := bytes32(0xaa)
	g1 := bytes32(0xbb)
	joiner := bytes32(0xcc)
	k.AcceptProof(0, g0, 10)
	k.AcceptProof(0, g1, 10)
	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: joiner, Weight: 10}))
	raw := k.buildLNPR(0)
	blob, ok := types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("join lnpr")
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	if n := countSetBits(k); n != 3 {
		t.Fatalf("after JOIN bits=%d want 3", n)
	}
	NoteMembershipBytes(types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: joiner}))
	raw = k.buildLNPR(0)
	blob, ok = types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("leave lnpr")
	}
	var sawZero bool
	for _, s := range blob.Subjects {
		if string(s.Subject) == string(joiner) && s.Weight == 0 {
			sawZero = true
		}
	}
	if !sawZero {
		t.Fatalf("LEAV must be weight-0 on LNPR: %+v", subjectsBrief(blob))
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	if n := countSetBits(k); n != 2 {
		t.Fatalf("after LEAV bits=%d want 2", n)
	}
	raw = k.buildLNPR(0)
	blob, ok = types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("post lnpr")
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	if n := countSetBits(k); n != 2 {
		t.Fatalf("JOIN file re-admitted leaver bits=%d want 2", n)
	}
	// Recheck of spent JOIN (Comet flood mempool) must not re-admit.
	k.NoteMembershipTx(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: joiner, Weight: 10}))
	raw = k.buildLNPR(0)
	blob, ok = types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("recheck lnpr")
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	if n := countSetBits(k); n != 2 {
		t.Fatalf("Recheck JOIN re-admitted leaver bits=%d want 2", n)
	}
}

func bytes32(fill byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = fill
	}
	return b
}
