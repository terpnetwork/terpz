package keeper

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Store-sourced roots: a proof bound to zeros must fail once the store has non-zero roots.
func TestStoreSourcedRootsBind(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	period := uint64(1)
	subj := []byte("ed25519-pubkey-bytes-32xx")
	w := int64(10)

	zeroProof := DummyStwoProveBoundRoots(period, subj, w, nil)
	blob := types.LNPRBlob{
		Period:   period,
		Subjects: []types.SubjectProof{{Subject: subj, Weight: w, Proof: zeroProof}},
	}
	if err := k.VerifyLNPR(blob); err != nil {
		t.Fatalf("zero roots default: %v", err)
	}

	roots := make([]byte, types.ObjectRootsSize)
	roots[0] = 0xAB
	k.SetLastObjectRoots(roots)
	if err := k.VerifyLNPR(blob); err == nil {
		t.Fatal("proof bound to zero roots must fail after store roots change")
	}

	okProof := DummyStwoProveBoundRoots(period, subj, w, k.LastObjectRoots())
	blob.Subjects[0].Proof = okProof
	if err := k.VerifyLNPR(blob); err != nil {
		t.Fatal(err)
	}
}

func TestRecheckSkipsCryptoKeepsEmptyReplace(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.AcceptProof(0, []byte("stay"), 10)
	empty := types.LNPRBlob{Period: 0, Subjects: nil}
	if err := k.VerifyLNPRRecheck(empty); err == nil {
		t.Fatal("recheck must still reject empty replace")
	}
	// uniqueness: dups rejected without calling ClosedVerifier
	dup := types.LNPRBlob{
		Period: 0,
		Subjects: []types.SubjectProof{
			{Subject: []byte("a"), Weight: 1, Proof: []byte("garbage")},
			{Subject: []byte("a"), Weight: 1, Proof: []byte("garbage")},
		},
	}
	if err := k.VerifyLNPRRecheck(dup); err == nil {
		t.Fatal("recheck must reject duplicate subject")
	}
}

func TestMempoolRejectsLNPR(t *testing.T) {
	blob := types.EncodeLNPR(types.LNPRBlob{Period: 1})
	if err := types.RejectMempoolLNPR(blob); err != types.ErrMempoolLNPR {
		t.Fatalf("got %v", err)
	}
	if err := types.RejectMempoolLNPR([]byte("normal-sdk-tx")); err != nil {
		t.Fatal(err)
	}
}

func TestProcessStillRejectsForgedRoots(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	roots := make([]byte, types.ObjectRootsSize)
	roots[31] = 1
	k.SetLastObjectRoots(roots)
	period := types.PeriodFromHeight(1)
	subj := []byte("ed25519-pubkey-bytes-32xx")
	// attacker proves against zeros
	proof := DummyStwoProveBound(period, subj, 10)
	lnpr := types.EncodeLNPR(types.LNPRBlob{
		Period:   period,
		Subjects: []types.SubjectProof{{Subject: subj, Weight: 10, Proof: proof}},
	})
	h := k.WrapProcessProposal(nil)
	resp, err := h(&abci.RequestProcessProposal{Height: 1, Txs: [][]byte{lnpr}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != abci.ResponseProcessProposal_REJECT {
		t.Fatalf("forged/zero-root Dummy must REJECT, got %v", resp.Status)
	}
}
