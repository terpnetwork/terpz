package keeper

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// LEAN-8 SSLE spec only (no circuit). Proposer id must stay hidden until the
// block that reveals it. Fail-closed until a hide-until-block schedule exists.

type sslePublicSchedule struct {
	period          uint64
	height          int64
	scheduledHidden []byte // commitment / ticket, not a consensus address
	revealedInBlock []byte // proposer id only after the block is produced
}

// UnimplementedSSLE is the fail-closed stub. Not a circuit.
type UnimplementedSSLE struct{}

func (UnimplementedSSLE) HiddenProposerID(period uint64, height int64) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (UnimplementedSSLE) RevealProposerAfterBlock(period uint64, height int64, block []byte) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestSSLE_ProposerIDHiddenUntilBlock(t *testing.T) {
	ssle := UnimplementedSSLE{}
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
}

func TestSSLE_RevealOnlyWithBlock(t *testing.T) {
	ssle := UnimplementedSSLE{}
	id, err := ssle.RevealProposerAfterBlock(3, 42, []byte("block-bytes"))
	if err != nil {
		t.Fatalf("SSLE reveal not implemented (red): %v", err)
	}
	if len(id) == 0 {
		t.Fatal("reveal after block must return proposer id")
	}
}

func TestSSLE_StubRejectsUnimplemented(t *testing.T) {
	err := error(nil)
	_, err = (UnimplementedSSLE{}).HiddenProposerID(1, 1)
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("UnimplementedSSLE must return not implemented, got %v", err)
	}
}
