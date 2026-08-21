package keeper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

// LEAN-3 Phase 1A valset AIR spec only (no circuit).
// On-chain row: 5-byte deposit index + 1-byte EB. Proofs must be real Stwo (STWO), not DSTW.

const (
	valsetIndexBits = 40
	valsetIndexMax  = uint64(1) << valsetIndexBits
)

type valsetAirSpecRow struct {
	period uint64
	index  uint64
	eb     uint8
	subj   []byte
}

func phase1AValsetSpecTable() []valsetAirSpecRow {
	return []valsetAirSpecRow{{
		period: 3,
		index:  42,
		eb:     32,
		subj:   []byte("lean-deposit-subject-32-bytes!!"),
	}}
}

// specValsetInstanceBytes is period(8 BE)|packed((index<<8)|eb)(8 BE)|subject.
func specValsetInstanceBytes(period, index uint64, eb uint8, subject []byte) []byte {
	out := make([]byte, 16+len(subject))
	binary.BigEndian.PutUint64(out[0:8], period)
	binary.BigEndian.PutUint64(out[8:16], (index<<8)|uint64(eb))
	copy(out[16:], subject)
	return out
}

// UnimplementedValsetAirSpec is fail-closed. Not a circuit.
type UnimplementedValsetAirSpec struct{}

func (UnimplementedValsetAirSpec) Prove(period, index uint64, eb uint8, subject []byte) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (UnimplementedValsetAirSpec) Verify(proof []byte, period, index uint64, eb uint8, subject []byte) error {
	return fmt.Errorf("not implemented")
}

func TestPhase1A_ValsetAIR_ValidStatementAccepted(t *testing.T) {
	air := StwoValsetAir{}
	for _, row := range phase1AValsetSpecTable() {
		proof, err := air.Prove(row.period, row.index, row.eb, row.subj)
		if err != nil {
			t.Skipf("lean-valset-air required: %v", err)
		}
		if len(proof) < 4 || !bytes.Equal(proof[:4], []byte("STWO")) {
			t.Fatalf("valset proof must be real Stwo (STWO), not DSTW: %x", proof)
		}
		if bytes.Equal(proof[:4], []byte("DSTW")) {
			t.Fatal("valset AIR must not use DummyStwo DSTW wire")
		}
		if err := air.Verify(proof, row.period, row.index, row.eb, row.subj); err != nil {
			t.Fatalf("valid valset statement must verify: %v", err)
		}
	}
}

func TestPhase1A_ValsetAIR_BitflipRejects(t *testing.T) {
	air := StwoValsetAir{}
	row := phase1AValsetSpecTable()[0]
	proof, err := air.Prove(row.period, row.index, row.eb, row.subj)
	if err != nil {
		t.Skipf("lean-valset-air required: %v", err)
	}
	flipped := append([]byte(nil), proof...)
	flipped[len(flipped)/2] ^= 1
	if err := air.Verify(flipped, row.period, row.index, row.eb, row.subj); err == nil {
		t.Fatal("bitflip of valset Stwo proof must reject")
	}
}

func TestPhase1A_ValsetStateIsFiveByteIndexPlusEB(t *testing.T) {
	row := phase1AValsetSpecTable()[0]
	if row.index >= valsetIndexMax {
		t.Fatal("fixture index must fit 5 bytes")
	}
	inst := specValsetInstanceBytes(0x0102030405060708, 0x0a0b0c0d0e, 0x20, []byte{0xaa})
	want := make([]byte, 17)
	binary.BigEndian.PutUint64(want[0:8], 0x0102030405060708)
	binary.BigEndian.PutUint64(want[8:16], (uint64(0x0a0b0c0d0e)<<8)|0x20)
	want[16] = 0xaa
	if !bytes.Equal(inst, want) {
		t.Fatalf("layout period(8 BE)|packed(index<<8|eb)(8 BE)|subject\n got %x\nwant %x", inst, want)
	}
}

func TestPhase1A_ValsetAIR_StubRejectsUnimplemented(t *testing.T) {
	_, err := (UnimplementedValsetAirSpec{}).Prove(1, 1, 1, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("UnimplementedValsetAirSpec must return not implemented, got %v", err)
	}
}

func TestPhase1A_ValsetAIR_DummyStwoNotSufficient(t *testing.T) {
	row := phase1AValsetSpecTable()[0]
	dummy := DummyStwoProveBound(row.period, row.subj, int64((row.index<<8)|uint64(row.eb)))
	if !bytes.HasPrefix(dummy, []byte("DSTW")) {
		t.Fatal("DummyStwo fixture must be DSTW")
	}
	if err := (UnimplementedValsetAirSpec{}).Verify(dummy, row.period, row.index, row.eb, row.subj); err == nil {
		t.Fatal("spec AIR must not accept DSTW DummyStwo as a valset proof")
	}
}

func TestPhase1A_MissingProofWeightZero(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	period := uint64(4)
	subj := []byte("no-valset-proof-subject-aaaaaa")
	k.PutSubject(period, subj, 100)
	set := k.BondedSet(period)
	if len(set) != 1 || set[0].Weight != 0 || set[0].HasProof {
		t.Fatalf("missing proof => weight 0, got %+v", set)
	}
}
