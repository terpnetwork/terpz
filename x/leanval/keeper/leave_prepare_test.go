package keeper

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Mirrors lean_production_e2e join_leav without Docker: JOIN then LEAV
// ride on LNPR via Prepare req.Txs (Comet flood analog). Recheck of the
// spent JOIN must not put the leaver back on the bitfield.
func TestLeaveInPrepareReqTxsClearsBitAndRecheckDoesNotRejoin(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.ClearPendingMembership()
	g0 := bytes32(0xa1)
	g1 := bytes32(0xa2)
	j0 := bytes32(0xb1)
	j1 := bytes32(0xb2)
	k.AcceptProof(0, g0, 10)
	k.AcceptProof(0, g1, 10)
	if n := countSetBits(k); n != 2 {
		t.Fatalf("genesis bits=%d want 2", n)
	}

	join0 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10})
	join1 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10})
	resp, err := k.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{
		Height: 4,
		Txs:    [][]byte{join0, join1},
	})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("JOIN LNPR missing")
	}
	if len(blob.Subjects) < 4 {
		t.Fatalf("JOIN LNPR subjects=%d want >=4: %+v", len(blob.Subjects), subjectsBrief(blob))
	}
	applyJoinTxs(t, k, resp.Txs)
	if n := countSetBits(k); n != 4 {
		t.Fatalf("after JOIN bits=%d want 4", n)
	}

	leave := types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: j1})
	resp, err = k.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{
		Height: 5,
		Txs:    [][]byte{leave},
	})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok = FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LEAV LNPR missing")
	}
	var sawZero bool
	for _, s := range blob.Subjects {
		if string(s.Subject) == string(j1) && s.Weight == 0 {
			sawZero = true
		}
	}
	if !sawZero {
		t.Fatalf("LEAV must be weight-0 on LNPR: %+v", subjectsBrief(blob))
	}
	applyJoinTxs(t, k, resp.Txs)
	if n := countSetBits(k); n != 3 {
		t.Fatalf("after LEAV bits=%d want 3", n)
	}
	if k.BitIsSet(depositMust(k, j1)) {
		t.Fatal("leaver bit must be clear")
	}

	// Comet Recheck of spent JOIN (still in flood mempool) + leftover LEAV.
	k.NoteMembershipTx(join1)
	k.NoteMembershipTx(leave)
	resp, err = k.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{
		Height: 6,
		Txs:    [][]byte{join1, leave},
	})
	if err != nil {
		t.Fatal(err)
	}
	applyJoinTxs(t, k, resp.Txs)
	if n := countSetBits(k); n != 3 {
		t.Fatalf("Recheck JOIN re-admitted leaver bits=%d want 3", n)
	}
}

func depositMust(k *Keeper, subject []byte) uint32 {
	idx, ok := k.depositIndexOf(subject)
	if !ok {
		panic("missing deposit index")
	}
	return idx
}
