package keeper

import (
	"bytes"
	"strings"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestLEAN5_HundredBondedDoesNotShipHundredRawStarks(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	for i := 0; i < 100; i++ {
		k.AcceptProof(0, bytes32(byte(i+1)), 10)
	}
	raw := k.buildLNPR(0)
	blob, ok := types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("LNPR")
	}
	if len(blob.Subjects) != 100 {
		t.Fatalf("subjects=%d want 100 (BondedSet inclusion)", len(blob.Subjects))
	}
	nProof := 0
	nFold := 0
	for _, s := range blob.Subjects {
		if len(s.Proof) == 0 {
			continue
		}
		nProof++
		if isFoldProof(s.Proof) {
			nFold++
		}
		if bytes.Contains(s.Proof, []byte("DSTW")) && nFold > 0 {
			t.Fatal("Dummy DSTW must not ride with a fold aggregate")
		}
	}
	if nProof > types.MaxRawStarksPerLNPR {
		t.Fatalf("raw proofs=%d exceeds MaxRawStarksPerLNPR=%d", nProof, types.MaxRawStarksPerLNPR)
	}
	if nProof >= 100 {
		t.Fatal("must not ship 100 raw STARKs per slot")
	}
}

func TestLEAN5_DummyDSTWInAggregateFailsBatch(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	a := bytes32(0x11)
	b := bytes32(0x22)
	k.AcceptProof(0, a, 10)
	k.AcceptProof(0, b, 10)
	roots := k.LastObjectRoots()
	pairs := foldPairStrings(0, nil, roots)
	fold, err := ProveSameStatementFold(pairs)
	if err != nil {
		if strings.Contains(err.Error(), "not on PATH") {
			t.Skip("lean-stwo-fold required")
		}
		t.Fatal(err)
	}
	blob := types.LNPRBlob{
		Period: 0,
		Subjects: []types.SubjectProof{
			{Subject: a, Weight: 10, Proof: fold},
			{Subject: b, Weight: 10, Proof: DummyStwoProveBoundRoots(0, b, 10, roots)},
		},
	}
	if err := k.VerifyLNPR(blob); err == nil {
		t.Fatal("Dummy DSTW in an aggregate must fail the batch")
	}
}

func TestLEAN5_DummyNIsNotAggregate(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	a := bytes32(0x31)
	k.AcceptProof(0, a, 10)
	dummy := DummyStwoProveBoundRoots(0, a, 10, k.LastObjectRoots())
	concat := append(append([]byte{}, dummy...), dummy...)
	if err := VerifySameStatementFold(concat, foldPairStrings(0, nil, k.LastObjectRoots())); err == nil {
		t.Fatal("Dummy-N concat must fail as a Stwo aggregate")
	}
}

func TestLEAN5_FoldIncludesRosterWithoutPerSubjectStark(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = false
	a := bytes32(0x41)
	b := bytes32(0x42)
	k.AcceptProof(0, a, 10)
	k.AcceptProof(0, b, 10)
	pairs := foldPairStrings(0, nil, k.LastObjectRoots())
	fold, err := ProveSameStatementFold(pairs)
	if err != nil {
		if strings.Contains(err.Error(), "not on PATH") {
			t.Skip("lean-stwo-fold required")
		}
		t.Fatal(err)
	}
	blob := types.LNPRBlob{
		Period: 0,
		Subjects: []types.SubjectProof{
			{Subject: a, Weight: 10, Proof: fold},
			{Subject: b, Weight: 10, Proof: nil}, // inclusion via bitfield/fold
		},
	}
	if err := k.VerifyLNPR(blob); err != nil {
		t.Fatalf("roster inclusion via fold: %v", err)
	}
	// JOIN extra with empty proof is not included in the current bitfield fold.
	extra := bytes32(0x43)
	blob.Subjects = append(blob.Subjects, types.SubjectProof{Subject: extra, Weight: 10, Proof: nil})
	if err := k.VerifyLNPR(blob); err == nil {
		t.Fatal("JOIN extra must not be treated as fold-included")
	}
}

func TestLEAN5_TooManyRawStarksRejected(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	subs := make([]types.SubjectProof, types.MaxRawStarksPerLNPR+1)
	for i := range subs {
		s := bytes32(byte(i + 1))
		k.AcceptProof(0, s, 10)
		subs[i] = types.SubjectProof{
			Subject: s,
			Weight:  10,
			Proof:   DummyStwoProveBoundRoots(0, s, 10, k.LastObjectRoots()),
		}
	}
	if err := k.VerifyLNPR(types.LNPRBlob{Period: 0, Subjects: subs}); err == nil {
		t.Fatal("over-cap raw Dummy STARKs must fail")
	}
}

func TestLEAN5_CheckTxStillRejectsMempoolLNPR(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.AcceptProof(0, bytes32(0x51), 10)
	raw := k.buildLNPR(0)
	if err := types.RejectMempoolLNPR(raw); err != types.ErrMempoolLNPR {
		t.Fatalf("CheckTx/mempool must still reject LNPR proofs: %v", err)
	}
	resp, err := k.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 2})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("JOIN/LEAV ride LNPR; LNPR missing from Prepare")
	}
	if len(blob.Subjects) < 1 {
		t.Fatal("LNPR subjects empty")
	}
}

func TestLEAN5_JoinLeaveStayLNPRSubjects(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	g0 := bytes32(0xa1)
	g1 := bytes32(0xa2)
	j0 := bytes32(0xb1)
	k.AcceptProof(0, g0, 10)
	k.AcceptProof(0, g1, 10)
	k.NoteMembershipTx(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10}))
	raw := k.buildLNPR(0)
	blob, ok := types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("LNPR")
	}
	found := false
	for _, s := range blob.Subjects {
		if bytes.Equal(s.Subject, j0) && s.Weight == 10 {
			found = true
		}
	}
	if !found {
		t.Fatal("JOIN must be an LNPR subject")
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatalf("JOIN apply: %v", err)
	}
	k.NoteMembershipTx(types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: j0}))
	raw = k.buildLNPR(0)
	blob, ok = types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("LEAV LNPR")
	}
	found = false
	for _, s := range blob.Subjects {
		if bytes.Equal(s.Subject, j0) && s.Weight == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("LEAV must be an LNPR subject (weight 0)")
	}
}
