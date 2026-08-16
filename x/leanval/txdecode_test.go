package leanval

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestCheckTxAcceptsJoin(t *testing.T) {
	raw := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: []byte("ed25519-pubkey-bytes-32xx"), Weight: 10})
	resp, err := CheckTx(func([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		t.Fatal("JOIN must not RunTx")
		return sdk.GasInfo{}, nil, nil, nil
	}, &abci.RequestCheckTx{Tx: raw})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Fatalf("code=%d log=%s", resp.Code, resp.Log)
	}
}

func TestCheckTxRejectsEmptyJoin(t *testing.T) {
	raw := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: nil, Weight: 10})
	resp, err := CheckTx(nil, &abci.RequestCheckTx{Tx: raw})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code == 0 {
		t.Fatal("empty subject must fail")
	}
}
