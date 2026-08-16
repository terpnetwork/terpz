package keeper

import (
	"strings"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Linus testnet gate: empty LNPR must not replace a live BondedSet
// (ApplyLNPR + dropUnlisted would otherwise zero Comet VP).

func TestApplyLNPR_EmptyReplaceRejected(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	period := uint64(1)
	subj := []byte("cons-addr-bbbbbbbbbbbbbbbb")
	w := int64(21)
	proof := DummyStwoProveBound(period, subj, w)
	blob := types.LNPRBlob{
		Period: period,
		Subjects: []types.SubjectProof{{
			Subject: subj,
			Weight:  w,
			Proof:   proof,
		}},
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatalf("seed apply: %v", err)
	}
	if got := k.BondedSet(period); len(got) != 1 || got[0].Weight != w {
		t.Fatalf("seed bonded: %+v", got)
	}

	empty := types.LNPRBlob{Period: period, Subjects: nil}
	err := k.ApplyLNPR(empty)
	if err == nil {
		t.Fatal("empty LNPR replace must be rejected")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error should mention empty, got %v", err)
	}
	got := k.BondedSet(period)
	if len(got) != 1 || got[0].Weight != w || !got[0].HasProof {
		t.Fatalf("empty replace must not wipe BondedSet: %+v", got)
	}
}

func TestVerifyLNPR_EmptyReplaceRejected(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	period := uint64(2)
	subj := []byte("cons-addr-cccccccccccccccc")
	k.AcceptProof(period, subj, 7)

	err := k.VerifyLNPR(types.LNPRBlob{Period: period, Subjects: []types.SubjectProof{}})
	if err == nil {
		t.Fatal("VerifyLNPR must reject empty replace of non-empty BondedSet")
	}
}

func TestCheckLNPR_EmptyReplaceProcessReject(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	period := uint64(0)
	subj := []byte("cons-addr-dddddddddddddddd")
	k.AcceptProof(period, subj, 3)
	tx := types.EncodeLNPR(types.LNPRBlob{Period: period, Subjects: nil})
	err := k.checkLNPR(1, [][]byte{tx})
	if err == nil {
		t.Fatal("ProcessProposal path must reject empty LNPR replace")
	}
}

func TestApplyLNPR_EmptyOkWhenBondedEmpty(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	if err := k.ApplyLNPR(types.LNPRBlob{Period: 0, Subjects: nil}); err != nil {
		t.Fatalf("empty LNPR on empty genesis period must apply as no-op: %v", err)
	}
	if got := k.BondedSet(0); len(got) != 0 {
		t.Fatalf("still empty: %+v", got)
	}
}

func TestVerifyLNPR_EmptyRejectedWhenPriorPeriodHasSet(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AcceptProof(0, []byte("cons-addr-eeeeeeeeeeeeeeee"), 9)
	err := k.VerifyLNPR(types.LNPRBlob{Period: 1, Subjects: nil})
	if err == nil {
		t.Fatal("empty LNPR must not replace a carried prior BondedSet")
	}
}
