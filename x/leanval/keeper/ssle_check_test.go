package keeper

import (
	"bytes"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestCheckSSLE_AbsentIsOptional(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	if err := k.checkSSLE(4, nil); err != nil {
		t.Fatalf("missing SSLE must not REJECT: %v", err)
	}
}

func TestCheckSSLE_DummyReject(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	tx := types.EncodeSSLE(types.SSLEBlob{
		Period:   0,
		Height:   4,
		Ticket:   bytes.Repeat([]byte{1}, 32),
		Proposer: bytes.Repeat([]byte{2}, 20),
		Proof:    []byte("DSTW"),
	})
	if err := k.checkSSLE(4, [][]byte{tx}); err == nil {
		t.Fatal("Dummy DSTW SSLE must Process REJECT")
	}
}

func TestCheckSSLE_WrongHeightReject(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	tx := types.EncodeSSLE(types.SSLEBlob{
		Period:   0,
		Height:   3,
		Ticket:   bytes.Repeat([]byte{1}, 32),
		Proposer: bytes.Repeat([]byte{2}, 20),
		Proof:    []byte("STWO"),
	})
	if err := k.checkSSLE(4, [][]byte{tx}); err == nil {
		t.Fatal("SSLE height != block height must REJECT")
	}
}

func TestCheckSSLE_WrongPeriodReject(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	tx := types.EncodeSSLE(types.SSLEBlob{
		Period:   9,
		Height:   4,
		Ticket:   bytes.Repeat([]byte{1}, 32),
		Proposer: bytes.Repeat([]byte{2}, 20),
		Proof:    []byte("STWO"),
	})
	if err := k.checkSSLE(4, [][]byte{tx}); err == nil {
		t.Fatal("SSLE period != height/600 must REJECT")
	}
}

func TestCheckSSLE_WrongKindReject(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	proof := append([]byte("STWO"), 2, 5)
	proof = append(proof, []byte("FOLD")...)
	proof = append(proof, make([]byte, 96)...)
	tx := types.EncodeSSLE(types.SSLEBlob{
		Period:   0,
		Height:   4,
		Ticket:   bytes.Repeat([]byte{1}, 32),
		Proposer: bytes.Repeat([]byte{2}, 20),
		Proof:    proof,
	})
	if err := k.checkSSLE(4, [][]byte{tx}); err == nil {
		t.Fatal("STWO FOLD costume must not verify as SSLE")
	}
}

func TestSSLE_StwoBitflipRejects(t *testing.T) {
	if ssleBin() == "" {
		t.Skip("lean-ssle not built")
	}
	proposer := bytes.Repeat([]byte{0xab}, 32)
	ticket, proof, err := ProveSSLE(3, 42, proposer)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySSLEProof(proof, 3, 42, ticket); err != nil {
		t.Fatal(err)
	}
	bad := append([]byte(nil), proof...)
	bad[len(bad)/2] ^= 0xff
	if err := VerifySSLEProof(bad, 3, 42, ticket); err == nil {
		t.Fatal("bitflipped SSLE proof must fail verify")
	}
}
