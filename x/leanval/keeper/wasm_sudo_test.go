package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type mockWasm struct {
	last []byte
	fail bool
}

func (m *mockWasm) Sudo(_ context.Context, _ sdk.AccAddress, msg []byte) ([]byte, error) {
	m.last = append([]byte(nil), msg...)
	if m.fail {
		return nil, fmt.Errorf("contract reject")
	}
	return []byte(`{"ok":true}`), nil
}

func TestVerifyProofUsesModuleSudoWhenBound(t *testing.T) {
	m := &mockWasm{}
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetWasmSudo(m)
	k.SetModuleVerifier([]byte("leanverifier___________"), 2)
	k.BindContext(sdk.Context{})
	proof := DummyStwoProve(1, 2)
	if err := k.verifyProof(proof, nil); err != nil {
		t.Fatal(err)
	}
	var msg SudoProofInstanceVerifyMsg
	if err := json.Unmarshal(m.last, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.ProofInstanceVerify.ZkID != 2 {
		t.Fatalf("zkid %d", msg.ProofInstanceVerify.ZkID)
	}
	if string(msg.ProofInstanceVerify.Proof[:4]) != "DSTW" {
		t.Fatalf("proof %x", msg.ProofInstanceVerify.Proof[:4])
	}
}

func TestVerifyProofFallsBackWithoutContract(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	if err := k.verifyProof(DummyStwoProve(3, 5), nil); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyProofSudoReject(t *testing.T) {
	m := &mockWasm{fail: true}
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.SetWasmSudo(m)
	k.SetModuleVerifier([]byte("x"), 1)
	k.BindContext(sdk.Context{})
	if err := k.verifyProof(DummyStwoProve(1, 1), nil); err == nil {
		t.Fatal("sudo fail must reject")
	}
}
