package keeper

import (
	"bytes"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestCommittedJoinThenPrepare(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	stay := bytes.Repeat([]byte{0xaa}, 32)
	join := bytes.Repeat([]byte{0xbb}, 32)
	k.AcceptProof(0, stay, 10)
	if err := k.ApplyJoin(types.JoinBlob{Period: 0, Subject: join, Weight: 7}); err != nil {
		t.Fatal(err)
	}
	h := k.WrapPrepareProposal(nil)
	resp, err := h(&abci.RequestPrepareProposal{Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("expected LNPR")
	}
	var sawStay, sawJoin bool
	for _, s := range blob.Subjects {
		if s.Subject[0] == 0xaa && s.Weight == 10 {
			sawStay = true
		}
		if s.Subject[0] == 0xbb && s.Weight == 7 {
			sawJoin = true
		}
	}
	if !sawStay || !sawJoin {
		t.Fatalf("committed join must appear in LNPR: %+v", blob.Subjects)
	}
}

func TestCommittedLeaveThenPrepare(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	stay := bytes.Repeat([]byte{0xaa}, 32)
	gone := bytes.Repeat([]byte{0xcc}, 32)
	k.AcceptProof(0, stay, 10)
	k.AcceptProof(0, gone, 10)
	if err := k.ApplyLeave(types.LeaveBlob{Period: 0, Subject: gone}); err != nil {
		t.Fatal(err)
	}
	h := k.WrapPrepareProposal(nil)
	resp, err := h(&abci.RequestPrepareProposal{Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("expected LNPR")
	}
	for _, s := range blob.Subjects {
		if s.Subject[0] == 0xcc {
			t.Fatalf("left subject still in LNPR: %+v", blob.Subjects)
		}
	}
	if len(blob.Subjects) != 1 || blob.Subjects[0].Subject[0] != 0xaa {
		t.Fatalf("want stay only: %+v", blob.Subjects)
	}
}

func TestTwoReplicasSameMembershipTxs(t *testing.T) {
	stay := bytes.Repeat([]byte{0x11}, 32)
	join := bytes.Repeat([]byte{0x22}, 32)
	apply := func() []SubjectPower {
		k := NewKeeper(NewMemStore(), DummyStwoGo{})
		k.AllowDummy = true
		k.AcceptProof(0, stay, 10)
		roots := k.LastObjectRoots()
		txs := [][]byte{
			types.EncodeLNPR(types.LNPRBlob{Period: 0, Subjects: []types.SubjectProof{
				{Subject: stay, Weight: 10, Proof: DummyStwoProveBoundRoots(0, stay, 10, roots)},
			}}),
			types.EncodeJoin(types.JoinBlob{Period: 0, Subject: join, Weight: 8}),
		}
		if err := k.ProcessInjectedLNPR(txs); err != nil {
			t.Fatal(err)
		}
		if err := k.ProcessMembershipTxs(txs); err != nil {
			t.Fatal(err)
		}
		return k.BondedSet(0)
	}
	a, b := apply(), apply()
	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("want 2 rows each: a=%+v b=%+v", a, b)
	}
	for i := range a {
		if !bytes.Equal(a[i].Subject, b[i].Subject) || a[i].Weight != b[i].Weight {
			t.Fatalf("replicas diverged: %+v vs %+v", a, b)
		}
	}
}

func TestProcessMembershipAfterLNPRKeepsJoin(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	stay := bytes.Repeat([]byte{0x11}, 32)
	join := bytes.Repeat([]byte{0x22}, 32)
	k.AcceptProof(0, stay, 10)
	roots := k.LastObjectRoots()
	txs := [][]byte{
		types.EncodeLNPR(types.LNPRBlob{Period: 0, Subjects: []types.SubjectProof{
			{Subject: stay, Weight: 10, Proof: DummyStwoProveBoundRoots(0, stay, 10, roots)},
		}}),
		types.EncodeJoin(types.JoinBlob{Period: 0, Subject: join, Weight: 8}),
	}
	if err := k.ProcessInjectedLNPR(txs); err != nil {
		t.Fatal(err)
	}
	if err := k.ProcessMembershipTxs(txs); err != nil {
		t.Fatal(err)
	}
	set := k.BondedSet(0)
	if len(set) != 2 {
		t.Fatalf("join after LNPR replace-set must survive: %+v", set)
	}
}

func TestStorePendingJoinSurvivesLNPRReplace(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.AcceptProof(0, []byte("genesis-ed25519-key-32bytesxxxx"), 10)
	join := []byte("joiner-ed25519-key-32bytesxxxxx")
	if err := k.ApplyJoin(types.JoinBlob{Period: 0, Subject: join, Weight: 8}); err != nil {
		t.Fatal(err)
	}
	k.ClearPendingMembership() // RAM gone; store pending must remain
	raw := k.buildLNPR(0)
	dec, ok := types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("lnpr")
	}
	if len(dec.Subjects) != 2 {
		t.Fatalf("subjects=%d want 2 after store pending", len(dec.Subjects))
	}
	// LNPR of genesis only must not wipe the pending join row
	gen := []byte("genesis-ed25519-key-32bytesxxxx")
	if err := k.ApplyLNPR(types.LNPRBlob{Period: 0, Subjects: []types.SubjectProof{{
		Subject: gen, Weight: 10,
		Proof: DummyStwoProveBoundRoots(0, gen, 10, k.LastObjectRoots()),
	}}}); err != nil {
		t.Fatal(err)
	}
	if n := len(k.QueryBondedSet(0)); n != 2 {
		t.Fatalf("BondedSet=%d want 2 (pending join survives dropUnlisted)", n)
	}
}

func TestPrepareAppendsStorePendingJoinTx(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	stay := bytes.Repeat([]byte{0xaa}, 32)
	join := bytes.Repeat([]byte{0xbb}, 32)
	k.AcceptProof(0, stay, 10)
	if err := k.ApplyJoin(types.JoinBlob{Period: 0, Subject: join, Weight: 8}); err != nil {
		t.Fatal(err)
	}
	k.ClearPendingMembership()
	h := k.WrapPrepareProposal(nil)
	if _, err := h(&abci.RequestPrepareProposal{Height: 1}); err != nil {
		t.Fatal(err)
	}
	if !k.BitIsSet(1) {
		t.Fatal("store join must set participation bit (not req.Txs)")
	}
	if n := len(k.QueryBondedSet(0)); n != 2 {
		t.Fatalf("BondedSet=%d want 2", n)
	}
}
