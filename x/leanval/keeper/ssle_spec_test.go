package keeper

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// LEAN-8 SSLE: proposer id must stay hidden until the block that reveals it.
// Stwo proof (prover_id=2 M31) is required; Dummy DSTW must fail.

type sslePublicSchedule struct {
	period          uint64
	height          int64
	scheduledHidden []byte
	revealedInBlock []byte
}

type UnimplementedSSLE struct{}

func (UnimplementedSSLE) HiddenProposerID(period uint64, height int64) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (UnimplementedSSLE) RevealProposerAfterBlock(period uint64, height int64, block []byte) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestSSLE_ProposerIDHiddenUntilBlock(t *testing.T) {
	ssle := HideUntilBlockSSLE{}
	row := sslePublicSchedule{
		period:          3,
		height:          42,
		scheduledHidden: []byte("ticket-not-ed25519-addr"),
		revealedInBlock: []byte("ed25519-proposer-pk-32-bytes!!!!"),
	}
	got, err := ssle.HiddenProposerID(row.period, row.height)
	if err != nil {
		t.Fatalf("SSLE hide-until-block not implemented (red): %v", err)
	}
	if bytes.Equal(got, row.revealedInBlock) {
		t.Fatal("scheduled view must not equal the proposer id")
	}
	if bytes.Contains(got, row.revealedInBlock) {
		t.Fatal("scheduled view must not embed the proposer id")
	}
	if len(got) != 32 {
		t.Fatalf("ticket len %d", len(got))
	}
}

func TestSSLE_RevealOnlyWithBlock(t *testing.T) {
	ssle := HideUntilBlockSSLE{}
	id, err := ssle.RevealProposerAfterBlock(3, 42, []byte("block-bytes"))
	if err != nil {
		t.Fatalf("SSLE reveal not implemented (red): %v", err)
	}
	if len(id) == 0 {
		t.Fatal("reveal after block must return proposer id")
	}
}

func TestSSLE_StubRejectsUnimplemented(t *testing.T) {
	_, err := (UnimplementedSSLE{}).HiddenProposerID(1, 1)
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("UnimplementedSSLE must return not implemented, got %v", err)
	}
}

func TestSSLE_DummyDSTWIsNotAProof(t *testing.T) {
	err := VerifySSLEProof([]byte("DSTWDSTW"), 1, 1, make([]byte, 32))
	if err == nil {
		t.Fatal("Dummy DSTW must not verify as SSLE")
	}
}

func TestSSLE_MempoolRejected(t *testing.T) {
	if err := types.RejectMempoolSSLE(append(append([]byte{}, types.PrefixSSLE...), 0x01)); err != types.ErrMempoolSSLE {
		t.Fatalf("got %v", err)
	}
	if err := types.RejectMempoolSSLE([]byte{0x0a}); err != nil {
		t.Fatal(err)
	}
}

func TestSSLE_StwoProveVerify_TicketNotProposer(t *testing.T) {
	if ssleBin() == "" {
		t.Skip("lean-ssle not built")
	}
	proposer := bytes.Repeat([]byte{0xab}, 32)
	ticket, proof, err := ProveSSLE(3, 42, proposer)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ticket, proposer) {
		t.Fatal("ticket must not equal proposer")
	}
	if err := VerifySSLEProof(proof, 3, 42, ticket); err != nil {
		t.Fatal(err)
	}
	blob := types.EncodeSSLE(types.SSLEBlob{Period: 3, Height: 42, Ticket: ticket, Proposer: proposer, Proof: proof})
	ssle := HideUntilBlockSSLE{}
	hidden, _ := ssle.HiddenProposerID(3, 42)
	if bytes.Equal(hidden, proposer) {
		t.Fatal("scheduled view leaked proposer")
	}
	got, err := ssle.RevealProposerAfterBlock(3, 42, blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, proposer) {
		t.Fatalf("reveal %x want %x", got, proposer)
	}
}
