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
