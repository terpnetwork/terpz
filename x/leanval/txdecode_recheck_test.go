package leanval

import (
	"bytes"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestWrapTxDecoderDoesNotNoteMembership(t *testing.T) {
	k := keeper.NewKeeper(keeper.NewMemStore(), keeper.DummyStwoGo{})
	k.ClearPendingMembership()
	raw := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: bytes.Repeat([]byte{0x11}, 32), Weight: 10})
	dec := WrapTxDecoder(func([]byte) (sdk.Tx, error) {
		t.Fatal("JOIN must not hit the inner decoder")
		return nil, nil
	})
	tx, err := dec(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tx.(types.MembershipTx); !ok {
		t.Fatalf("want MembershipTx, got %T", tx)
	}
	if n := len(k.PendingMembershipTxs()); n != 0 {
		t.Fatalf("WrapTxDecoder Recheck must not Note, pending=%d", n)
	}
}

func TestCheckTxDoesNotNoteMembership(t *testing.T) {
	k := keeper.NewKeeper(keeper.NewMemStore(), keeper.DummyStwoGo{})
	k.ClearPendingMembership()
	raw := types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: bytes.Repeat([]byte{0x22}, 32)})
	resp, err := CheckTx(func([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		t.Fatal("LEAV must not RunTx")
		return sdk.GasInfo{}, nil, nil, nil
	}, &abci.RequestCheckTx{Tx: raw})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Fatalf("code=%d log=%s", resp.Code, resp.Log)
	}
	if n := len(k.PendingMembershipTxs()); n != 0 {
		t.Fatalf("CheckTx Recheck must not Note, pending=%d", n)
	}
}

func TestNewCheckTxNotesLeaveSkipsSpentJoin(t *testing.T) {
	k := keeper.NewKeeper(keeper.NewMemStore(), keeper.DummyStwoGo{})
	k.AllowDummy = true
	k.ClearPendingMembership()
	joiner := bytes.Repeat([]byte{0x33}, 32)
	k.AcceptProof(0, joiner, 10)
	h := NewCheckTx(k)

	join := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: joiner, Weight: 10})
	resp, err := h(nil, &abci.RequestCheckTx{Tx: join})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Fatalf("spent JOIN Recheck code=%d", resp.Code)
	}
	if n := len(k.PendingMembershipTxs()); n != 0 {
		t.Fatalf("spent JOIN Recheck must not queue, pending=%d", n)
	}

	leave := types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: joiner})
	resp, err = h(nil, &abci.RequestCheckTx{Tx: leave})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Fatalf("LEAV CheckTx code=%d log=%s", resp.Code, resp.Log)
	}
	if n := len(k.PendingMembershipTxs()); n != 1 {
		t.Fatalf("LEAV must queue, pending=%d", n)
	}
}
