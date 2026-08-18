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
