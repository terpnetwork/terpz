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
