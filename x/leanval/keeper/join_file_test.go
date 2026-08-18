package keeper

import (
	"os"
	"path/filepath"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestFilePersistReachesLNPRAfterRAMReset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEANVAL_MEMBERSHIP_DISABLE", "")
	t.Setenv("LEANVAL_MEMBERSHIP_DIR", dir)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.AcceptProof(0, []byte("genesis-ed25519-key-32bytesxxxx"), 10)
	join := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: []byte("joiner-ed25519-key-32bytesxxxxx"), Weight: 10})
	NoteMembershipBytes(join)
	ents, _ := os.ReadDir(dir)
	if len(ents) == 0 {
		t.Fatal("expected JOIN file on disk")
	}
	// Simulate CLI NewKeeper RAM wipe; files must still feed Prepare.
	k2 := NewKeeper(k.store, DummyStwoGo{})
	k2.AllowDummy = true
	h := k2.WrapPrepareProposal(func(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		return &abci.ResponsePrepareProposal{Txs: req.Txs}, nil
	})
	resp, err := h(&abci.RequestPrepareProposal{Height: 3, Txs: nil})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(blob.Subjects) < 2 {
		t.Fatalf("lnpr subjects=%d want >=2 after file persist; dir=%s nfiles=%d pending=%d",
			len(blob.Subjects), dir, len(ents), len(k2.PendingMembershipTxs()))
	}
	_ = filepath.Separator
}
