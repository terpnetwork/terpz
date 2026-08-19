package keeper

import (
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestForgetJoinKeepsPendingLeave(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.ClearPendingMembership()
	subj := bytes32(0xcc)
	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: subj, Weight: 10}))
	NoteMembershipBytes(types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: subj}))
	if n := len(k.PendingMembershipTxs()); n != 2 {
		t.Fatalf("pending=%d want 2", n)
	}
	forgetMembershipJoin(subj)
	got := k.PendingMembershipTxs()
	if len(got) != 1 {
		t.Fatalf("after forget JOIN pending=%d want 1 (LEAV)", len(got))
	}
	if _, ok := types.DecodeLeave(got[0]); !ok {
		t.Fatal("remaining pending must be LEAV")
	}
}

// Roster-refresh Apply (current bits, all weight>0) must not delete a pending
// LEAV. Live e2e hole: empty blocks after JOIN called forget(j1) and wiped
// the leave file before the next Prepare could put weight 0 on LNPR.
func TestRosterRefreshApplyKeepsPendingLeave(t *testing.T) {
	a := NewKeeper(NewMemStore(), DummyStwoGo{})
	a.AllowDummy = true
	a.ClearPendingMembership()
	g0 := bytes32(0xa1)
	g1 := bytes32(0xa2)
	joiner := bytes32(0xb2)
	a.AcceptProof(0, g0, 10)
	a.AcceptProof(0, g1, 10)
	join := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: joiner, Weight: 10})
	a.NoteMembershipTx(join)
	raw := a.buildLNPR(0)
	blob, ok := types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("JOIN LNPR")
	}
	if err := a.ApplyLNPR(blob); err != nil {
		t.Fatalf("a JOIN: %v", err)
	}
	if n := countSetBits(a); n != 3 {
		t.Fatalf("after JOIN bits=%d want 3", n)
	}

	// Snapshot a weight>0 roster LNPR before LEAV is queued (empty-block analog).
	refresh := a.buildLNPR(0)
	refBlob, ok := types.DecodeLNPR(refresh)
	if !ok {
		t.Fatal("refresh lnpr")
	}

	leave := types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: joiner})
	a.NoteMembershipTx(leave)
	if n := len(a.PendingMembershipTxs()); n != 1 {
		t.Fatalf("pending LEAV=%d want 1", n)
	}
	for _, s := range refBlob.Subjects {
		if s.Weight == 0 {
			t.Fatal("refresh LNPR must be weight>0 roster")
		}
	}
	if err := a.ApplyLNPR(refBlob); err != nil {
		t.Fatalf("refresh ApplyLNPR: %v", err)
	}
	if n := len(a.PendingMembershipTxs()); n != 1 {
		t.Fatalf("roster refresh wiped pending LEAV, pending=%d want 1", n)
	}

	raw = a.buildLNPR(0)
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
		t.Fatalf("LEAV must survive refresh onto LNPR: %+v", subjectsBrief(blob))
	}
	if err := a.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	if n := countSetBits(a); n != 2 {
		t.Fatalf("after LEAV bits=%d want 2", n)
	}
}
