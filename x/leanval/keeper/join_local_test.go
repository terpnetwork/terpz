package keeper

import (
	"bytes"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Mirrors lean_production_e2e join_leav without Docker:
// CheckTx-ok JOIN must appear in the next LNPR and flip bits after ApplyLNPR.
func TestCheckTxJoinThenPrepareLNPRSetsBits(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.ClearPendingMembership()
	g0 := bytes.Repeat([]byte{0xa1}, 32)
	g1 := bytes.Repeat([]byte{0xa2}, 32)
	j0 := bytes.Repeat([]byte{0xb1}, 32)
	j1 := bytes.Repeat([]byte{0xb2}, 32)
	k.AcceptProof(0, g0, 10)
	k.AcceptProof(0, g1, 10)
	if n := bitCount(k); n != 2 {
		t.Fatalf("genesis bits=%d want 2", n)
	}

	raw0 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10})
	raw1 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10})
	k.NoteMembershipTx(raw0)
	k.NoteMembershipTx(raw1)
	if n := len(k.PendingMembershipTxs()); n != 2 {
		t.Fatalf("pending=%d want 2 (CheckTx must Note JOIN)", n)
	}

	prep := k.WrapPrepareProposal(nil)
	resp, err := prep(&abci.RequestPrepareProposal{Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(blob.Subjects) < 4 {
		t.Fatalf("LNPR subjects=%d want >=4 (JOIN must ride on LNPR): %+v", len(blob.Subjects), subjectsBrief(blob))
	}

	if err := k.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatalf("ApplyLNPR: %v", err)
	}
	if n := bitCount(k); n < 4 {
		t.Fatalf("bits after ApplyLNPR=%d want >=4", n)
	}
}

func TestPrepareWithoutNoteDoesNotInventJoin(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.ClearPendingMembership()
	k.AcceptProof(0, bytes.Repeat([]byte{0xa1}, 32), 10)
	k.AcceptProof(0, bytes.Repeat([]byte{0xa2}, 32), 10)
	resp, err := k.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 3})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR")
	}
	if len(blob.Subjects) != 2 {
		t.Fatalf("no JOIN noted: subjects=%d", len(blob.Subjects))
	}
}

// JOIN in RequestPrepareProposal.Txs is the Comet reap analog: no prior Note.
// Admission is LNPR subjects, not Deliver of the JOIN bytes.
func TestJoinInPrepareReqTxsWithoutNoteReachesLNPR(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.ClearPendingMembership()
	g0 := bytes.Repeat([]byte{0xa1}, 32)
	g1 := bytes.Repeat([]byte{0xa2}, 32)
	j0 := bytes.Repeat([]byte{0xc1}, 32)
	j1 := bytes.Repeat([]byte{0xc2}, 32)
	k.AcceptProof(0, g0, 10)
	k.AcceptProof(0, g1, 10)
	if n := bitCount(k); n != 2 {
		t.Fatalf("genesis bits=%d want 2", n)
	}
	raw0 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10})
	raw1 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10})
	if n := len(k.PendingMembershipTxs()); n != 0 {
		t.Fatalf("pending=%d want 0 (no CheckTx Note)", n)
	}
	resp, err := k.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{
		Height: 4,
		Txs:    [][]byte{raw0, raw1},
	})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(blob.Subjects) < 4 {
		t.Fatalf("LNPR subjects=%d want >=4 (JOIN must ride on LNPR from req.Txs): %+v", len(blob.Subjects), subjectsBrief(blob))
	}
	if err := k.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatalf("ApplyLNPR: %v", err)
	}
	if n := bitCount(k); n < 4 {
		t.Fatalf("bits after ApplyLNPR=%d want >=4", n)
	}
}

// Proposer Prepare output applied on a replica that never Note'd must set bits on BOTH.
func TestProposerJoinLNPRAppliedOnReplicaSetsBitsBoth(t *testing.T) {
	proposer := NewKeeper(NewMemStore(), DummyStwoGo{})
	replica := NewKeeper(NewMemStore(), DummyStwoGo{})
	proposer.AllowDummy = true
	replica.AllowDummy = true
	proposer.ClearPendingMembership()
	g0 := bytes.Repeat([]byte{0xd1}, 32)
	g1 := bytes.Repeat([]byte{0xd2}, 32)
	j0 := bytes.Repeat([]byte{0xe1}, 32)
	j1 := bytes.Repeat([]byte{0xe2}, 32)
	for _, k := range []*Keeper{proposer, replica} {
		k.AcceptProof(0, g0, 10)
		k.AcceptProof(0, g1, 10)
	}
	raw0 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10})
	raw1 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10})
	proposer.NoteMembershipTx(raw0)
	proposer.NoteMembershipTx(raw1)
	resp, err := proposer.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(blob.Subjects) < 4 {
		t.Fatalf("proposer LNPR subjects=%d want >=4", len(blob.Subjects))
	}
	if err := proposer.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatalf("proposer ApplyLNPR: %v", err)
	}
	if err := replica.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatalf("replica ApplyLNPR: %v", err)
	}
	pb, rb := bitCount(proposer), bitCount(replica)
	if pb < 4 || rb < 4 {
		t.Fatalf("bits proposer=%d replica=%d want both >=4", pb, rb)
	}
}

// Pending JOIN must not be applied in PreBlock: only LNPR subjects commit.
// Local ApplyJoin before consensus would diverge app-hash vs a replica that
// never saw CheckTx.
func TestPreBlockMustNotApplyPendingJoin(t *testing.T) {
	proposer := NewKeeper(NewMemStore(), DummyStwoGo{})
	replica := NewKeeper(NewMemStore(), DummyStwoGo{})
	proposer.AllowDummy = true
	replica.AllowDummy = true
	proposer.ClearPendingMembership()
	g0 := bytes.Repeat([]byte{0xf1}, 32)
	j0 := bytes.Repeat([]byte{0xf2}, 32)
	for _, k := range []*Keeper{proposer, replica} {
		k.AcceptProof(0, g0, 10)
	}
	raw := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10})
	proposer.NoteMembershipTx(raw)
	if bitCount(proposer) != bitCount(replica) {
		t.Fatal("Note must not flip bits (PreBlock must not ApplyJoin pending)")
	}
	if bitCount(proposer) != 1 {
		t.Fatalf("genesis bits=%d want 1", bitCount(proposer))
	}
	resp, err := proposer.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 3})
	if err != nil {
		t.Fatal(err)
	}
	if err := replica.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatal(err)
	}
	if err := proposer.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatal(err)
	}
	if bitCount(proposer) != bitCount(replica) || bitCount(replica) < 2 {
		t.Fatalf("after LNPR bits proposer=%d replica=%d want equal >=2", bitCount(proposer), bitCount(replica))
	}
}

func TestJoinMembershipTxGetMsgsEmptyNotFinalizeSdkTx(t *testing.T) {
	raw := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: bytes.Repeat([]byte{0x33}, 32), Weight: 10})
	tx := types.MembershipTx{Raw: raw}
	if msgs := tx.GetMsgs(); len(msgs) != 0 {
		t.Fatalf("GetMsgs=%d; JOIN must not Deliver as sdk tx", len(msgs))
	}
}

func bitCount(k *Keeper) int {
	n := 0
	for i := uint32(0); i < k.nextDepositIndex()+8; i++ {
		if k.BitIsSet(i) {
			n++
		}
	}
	return n
}

func subjectsBrief(blob types.LNPRBlob) []int {
	out := make([]int, len(blob.Subjects))
	for i, s := range blob.Subjects {
		out[i] = len(s.Subject)
	}
	return out
}
