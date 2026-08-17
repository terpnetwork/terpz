package keeper

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestProcessProposalRejectsOmittedLNPR(t *testing.T) {
	k := NewKeeper(NewMemStore(), TestVerifier{Accept: []byte("ok")})
	h := k.WrapProcessProposal(func(req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		t.Fatal("inner must not run on lean reject")
		return nil, nil
	})
	resp, err := h(&abci.RequestProcessProposal{Height: 1, Txs: [][]byte{[]byte("user-tx")}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != abci.ResponseProcessProposal_REJECT {
		t.Fatalf("status=%v", resp.Status)
	}
}

func TestProcessProposalRejectsBadVerifyDummy(t *testing.T) {
	k := NewKeeper(NewMemStore(), TestVerifier{Accept: []byte("good")})
	blob := types.EncodeLNPR(types.LNPRBlob{
		Period: types.PeriodFromHeight(1),
		Subjects: []types.SubjectProof{{
			Subject: []byte("v1"),
			Weight:  10,
			Proof:   []byte("bitflip"),
		}},
	})
	h := k.WrapProcessProposal(nil)
	resp, err := h(&abci.RequestProcessProposal{Height: 1, Txs: [][]byte{blob}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != abci.ResponseProcessProposal_REJECT {
		t.Fatalf("want REJECT on verify fail, got %v", resp.Status)
	}
}

func TestProcessProposalAcceptsValidLNPRAndHMVEOrder(t *testing.T) {
	k := NewKeeper(NewMemStore(), TestVerifier{Accept: []byte("good")})
	hmve := append(append([]byte{}, types.PrefixHMVE...), 0x01)
	lnpr := types.EncodeLNPR(types.LNPRBlob{
		Period: types.PeriodFromHeight(10),
		Subjects: []types.SubjectProof{{
			Subject: []byte("ed25519-pubkey-bytes-32xx"),
			Weight:  7,
			Proof:   []byte("good"),
		}},
	})
	var innerSaw [][]byte
	h := k.WrapProcessProposal(func(req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		innerSaw = req.Txs
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	})
	resp, err := h(&abci.RequestProcessProposal{Height: 10, Txs: [][]byte{hmve, lnpr}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != abci.ResponseProcessProposal_ACCEPT {
		t.Fatalf("status=%v", resp.Status)
	}
	if len(innerSaw) != 2 || !types.HasPrefix(innerSaw[0], types.PrefixHMVE) {
		t.Fatalf("HMVE stays txs[0]: %x", innerSaw)
	}

	if err := k.ProcessInjectedLNPR([][]byte{hmve, lnpr}); err != nil {
		t.Fatal(err)
	}
	set := k.QueryBondedSet(types.PeriodFromHeight(10))
	if len(set) != 1 || set[0].Weight != 7 {
		t.Fatalf("%+v", set)
	}
}

func TestPrepareInjectsLNPRAfterHMVE(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	hmInner := func(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		hmve := append(append([]byte{}, types.PrefixHMVE...), 0xaa)
		return &abci.ResponsePrepareProposal{Txs: [][]byte{hmve, []byte("user")}}, nil
	}
	h := k.WrapPrepareProposal(hmInner)
	resp, err := h(&abci.RequestPrepareProposal{Height: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Txs) < 2 {
		t.Fatalf("txs=%d", len(resp.Txs))
	}
	if !types.HasPrefix(resp.Txs[0], types.PrefixHMVE) {
		t.Fatal("txs[0] must remain HMVE")
	}
	if !types.HasPrefix(resp.Txs[1], types.PrefixLNPR) {
		t.Fatalf("txs[1] must be LNPR, got %x", resp.Txs[1][:4])
	}
}

func TestPrepareIncludesCometMembershipTxs(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	join := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: []byte("new-ed25519"), Weight: 10})
	h := k.WrapPrepareProposal(func(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		return &abci.ResponsePrepareProposal{Txs: req.Txs}, nil
	})
	resp, err := h(&abci.RequestPrepareProposal{Height: 3, Txs: [][]byte{join}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tx := range resp.Txs {
		if types.IsMembershipTx(tx) {
			found = true
		}
	}
	if !found {
		t.Fatalf("JOIN from Comet req.Txs must be in proposal: %d txs", len(resp.Txs))
	}
}

func TestProcessAcceptsLNPRThenJoin(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.AcceptProof(0, []byte("genesis-ed25519-key-32bytesxxxx"), 10)
	join := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: []byte("joiner-ed25519-key-32bytesxxxxx"), Weight: 10})
	prep := k.WrapPrepareProposal(func(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		return &abci.ResponsePrepareProposal{Txs: req.Txs}, nil
	})
	prepResp, err := prep(&abci.RequestPrepareProposal{Height: 4, Txs: [][]byte{join}})
	if err != nil {
		t.Fatal(err)
	}
	proc := k.WrapProcessProposal(nil)
	got, err := proc(&abci.RequestProcessProposal{Height: 4, Txs: prepResp.Txs})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != abci.ResponseProcessProposal_ACCEPT {
		t.Fatalf("Process REJECT with JOIN+LNPR (%d txs)", len(prepResp.Txs))
	}
	if err := k.ProcessInjectedLNPR(prepResp.Txs); err != nil {
		t.Fatal(err)
	}
	if err := k.ProcessMembershipTxs(prepResp.Txs); err != nil {
		t.Fatal(err)
	}
	if n := len(k.QueryBondedSet(0)); n != 2 {
		t.Fatalf("BondedSet rows=%d want 2", n)
	}
}

func TestBuildLNPRIncludesQueuedJoin(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.AcceptProof(0, []byte("genesis-ed25519-key-32bytesxxxx"), 10)
	join := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: []byte("joiner-ed25519-key-32bytesxxxxx"), Weight: 8})
	k.NoteMembershipTx(join)
	blob := types.EncodeLNPR(types.LNPRBlob{}) // placeholder
	_ = blob
	raw := k.buildLNPR(0)
	decoded, ok := types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("lnpr")
	}
	if len(decoded.Subjects) != 2 {
		t.Fatalf("subjects=%d want 2 (genesis+join)", len(decoded.Subjects))
	}
}
