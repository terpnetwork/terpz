package keeper

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestDummyStwoValidAndBitflip(t *testing.T) {
	p := DummyStwoProve(11, 22)
	if err := (DummyStwoGo{}).VerifyDummy(p, nil); err != nil {
		t.Fatal(err)
	}
	p[14] ^= 1
	if err := (DummyStwoGo{}).VerifyDummy(p, nil); err == nil {
		t.Fatal("bitflip must fail")
	}
}

func TestDummyStwoWrongProverID(t *testing.T) {
	p := DummyStwoProve(1, 2)
	p[4] = 0
	if err := (DummyStwoGo{}).VerifyDummy(p, nil); err == nil {
		t.Fatal("want fail closed")
	}
}

func TestDummyStwoForgedWeightFails(t *testing.T) {
	period := uint64(3)
	subj := []byte("ed25519-pubkey-bytes-32xx")
	proof := DummyStwoProveBound(period, subj, 10)
	instOK := instancesFor(period, subj, 10)
	if err := (DummyStwoGo{}).VerifyDummy(proof, instOK); err != nil {
		t.Fatal(err)
	}
	instBad := instancesFor(period, subj, 999)
	if err := (DummyStwoGo{}).VerifyDummy(proof, instBad); err == nil {
		t.Fatal("forged weight must fail")
	}

	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	blob := types.EncodeLNPR(types.LNPRBlob{
		Period: types.PeriodFromHeight(1),
		Subjects: []types.SubjectProof{{
			Subject: subj,
			Weight:  999,
			Proof:   DummyStwoProveBound(types.PeriodFromHeight(1), subj, 10),
		}},
	})
	h := k.WrapProcessProposal(nil)
	resp, err := h(&abci.RequestProcessProposal{Height: 1, Txs: [][]byte{blob}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != abci.ResponseProcessProposal_REJECT {
		t.Fatalf("forged weight LNPR must REJECT, got %v", resp.Status)
	}
}

func TestDummyJoinExtraRejected(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	g := make([]byte, 32)
	g[0] = 1
	j := make([]byte, 32)
	j[0] = 2
	k.AcceptProof(0, g, 10)
	roots := k.LastObjectRoots()
	blob := types.LNPRBlob{
		Period: 0,
		Subjects: []types.SubjectProof{
			{Subject: g, Weight: 10, Proof: DummyStwoProveBoundRoots(0, g, 10, roots)},
			{Subject: j, Weight: 10, Proof: DummyStwoProveBoundRoots(0, j, 10, roots)},
		},
	}
	if err := k.VerifyLNPR(blob); err == nil {
		t.Fatal("Dummy DSTW JOIN extra must reject")
	}
}
