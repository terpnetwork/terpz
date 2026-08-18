package leanval

import (
	"bytes"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestReapWithoutInsertIsEmpty(t *testing.T) {
	k := keeper.NewKeeper(keeper.NewMemStore(), keeper.DummyStwoGo{})
	k.ClearPendingMembership()
	reap := NewReapTxs(k)
	resp, err := reap(&abci.RequestReapTxs{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Txs) != 0 {
		t.Fatalf("empty queue reaped %d txs", len(resp.Txs))
	}
}

func TestInsertTwoJoinsThenReapReturnsBoth(t *testing.T) {
	k := keeper.NewKeeper(keeper.NewMemStore(), keeper.DummyStwoGo{})
	k.ClearPendingMembership()
	insert := NewInsertTx(k)
	reap := NewReapTxs(k)

	raw0 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: bytes.Repeat([]byte{0xb1}, 32), Weight: 10})
	raw1 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: bytes.Repeat([]byte{0xb2}, 32), Weight: 10})
	if _, err := insert(&abci.RequestInsertTx{Tx: raw0}); err != nil {
		t.Fatal(err)
	}
	if _, err := insert(&abci.RequestInsertTx{Tx: raw1}); err != nil {
		t.Fatal(err)
	}

	resp, err := reap(&abci.RequestReapTxs{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Txs) != 2 {
		t.Fatalf("reap txs=%d want 2 (empty stub must fail)", len(resp.Txs))
	}
	if !bytes.Equal(resp.Txs[0], raw0) || !bytes.Equal(resp.Txs[1], raw1) {
		t.Fatalf("reap order/bytes mismatch")
	}
}

// After Insert+Reap, Comet may still call Prepare with empty req.Txs
// (Reap is the only mempool handoff). JOIN must already be in the
// proposer pending queue so LNPR subjects >= 4. Replica never Notes;
// ProcessInjectedLNPR applies subjects on both keepers.
func TestInsertReapThenPrepareEmptyTxsLNPROnReplica(t *testing.T) {
	proposer := keeper.NewKeeper(keeper.NewMemStore(), keeper.DummyStwoGo{})
	replica := keeper.NewKeeper(keeper.NewMemStore(), keeper.DummyStwoGo{})
	proposer.AllowDummy = true
	replica.AllowDummy = true
	proposer.ClearPendingMembership()

	g0 := bytes.Repeat([]byte{0xa1}, 32)
	g1 := bytes.Repeat([]byte{0xa2}, 32)
	j0 := bytes.Repeat([]byte{0xb1}, 32)
	j1 := bytes.Repeat([]byte{0xb2}, 32)
	for _, k := range []*keeper.Keeper{proposer, replica} {
		k.AcceptProof(0, g0, 10)
		k.AcceptProof(0, g1, 10)
	}

	insert := NewInsertTx(proposer)
	reap := NewReapTxs(proposer)
	raw0 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10})
	raw1 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10})
	if _, err := insert(&abci.RequestInsertTx{Tx: raw0}); err != nil {
		t.Fatal(err)
	}
	if _, err := insert(&abci.RequestInsertTx{Tx: raw1}); err != nil {
		t.Fatal(err)
	}
	reaped, err := reap(&abci.RequestReapTxs{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped.Txs) != 2 {
		t.Fatalf("reap txs=%d want 2", len(reaped.Txs))
	}

	// Empty req.Txs: RAM Note from Insert is proposer-local, not a replica protocol.
	resp, err := proposer.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := keeper.FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(blob.Subjects) < 4 {
		t.Fatalf("LNPR subjects=%d want >=4 after Insert/Reap with empty Prepare req.Txs", len(blob.Subjects))
	}

	if err := proposer.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatalf("proposer ApplyLNPR: %v", err)
	}
	if err := replica.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatalf("replica ApplyLNPR: %v", err)
	}
	pb, rb := bitCountKeeper(proposer), bitCountKeeper(replica)
	if pb < 4 || rb < 4 {
		t.Fatalf("bits proposer=%d replica=%d want both >=4", pb, rb)
	}
}

func bitCountKeeper(k *keeper.Keeper) int {
	n := 0
	for i := uint32(0); i < 16; i++ {
		if k.BitIsSet(i) {
			n++
		}
	}
	return n
}
