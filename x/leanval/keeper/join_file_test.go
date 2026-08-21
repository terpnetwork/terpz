package keeper

import (
	"bytes"
	"encoding/json"
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

func TestJoinFilesApplyLNPRBitsBothThenLeaveDrops(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEANVAL_MEMBERSHIP_DISABLE", "")
	t.Setenv("LEANVAL_MEMBERSHIP_DIR", dir)

	proposer := NewKeeper(NewMemStore(), DummyStwoGo{})
	replica := NewKeeper(NewMemStore(), DummyStwoGo{})
	proposer.AllowDummy = true
	replica.AllowDummy = true

	g0 := bytes.Repeat([]byte{0xa1}, 32)
	g1 := bytes.Repeat([]byte{0xa2}, 32)
	j0 := bytes.Repeat([]byte{0xb1}, 32)
	j1 := bytes.Repeat([]byte{0xb2}, 32)
	for _, k := range []*Keeper{proposer, replica} {
		k.AcceptProof(0, g0, 10)
		k.AcceptProof(0, g1, 10)
	}
	if n := bitCount(proposer); n != 2 {
		t.Fatalf("genesis bits=%d want 2", n)
	}

	raw0 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10})
	raw1 := types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10})
	NoteMembershipBytes(raw0)
	NoteMembershipBytes(raw1)

	// NewKeeper wipes RAM; JOIN .tx files must still become LNPR subjects.
	store := proposer.Store()
	proposer = NewKeeper(store, DummyStwoGo{})
	proposer.AllowDummy = true
	pending := proposer.PendingMembershipTxs()
	if len(pending) < 2 {
		t.Fatalf("pending=%d want >=2 from JOIN files after RAM reset", len(pending))
	}

	resp, err := proposer.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(pending) >= 2 && len(blob.Subjects) < 4 {
		t.Fatalf("pending=%d lnpr_subjects=%d: genesis-only LNPR while JOIN files exist",
			len(pending), len(blob.Subjects))
	}
	if !lnprHasSubject(blob, j0) || !lnprHasSubject(blob, j1) {
		t.Fatalf("LNPR must include JOIN file subjects: %+v", subjectsBrief(blob))
	}

	rec := readLastPrepare(t, dir)
	if rec.Pending < 2 {
		t.Fatalf("last-prepare pending=%d want >=2", rec.Pending)
	}
	if rec.LNPRSubjects < 4 {
		t.Fatalf("last-prepare lnpr_subjects=%d want >=4", rec.LNPRSubjects)
	}

	applyJoinTxs(t, proposer, resp.Txs)
	applyJoinTxs(t, replica, resp.Txs)
	pb, rb := bitCount(proposer), bitCount(replica)
	if pb < 4 || rb < 4 {
		t.Fatalf("bits proposer=%d replica=%d want both >=4", pb, rb)
	}

	leav := types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: j1})
	NoteMembershipBytes(leav)
	resp2, err := proposer.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 5})
	if err != nil {
		t.Fatal(err)
	}
	blob2, _, ok := FindLNPR(resp2.Txs)
	if !ok {
		t.Fatal("LNPR missing after LEAV")
	}
	var leaveW int64 = -1
	for _, s := range blob2.Subjects {
		if bytes.Equal(s.Subject, j1) {
			leaveW = s.Weight
		}
	}
	if leaveW != 0 {
		t.Fatalf("LEAV must be an LNPR subject with weight 0, got %d subjects=%+v", leaveW, subjectsBrief(blob2))
	}
	applyJoinTxs(t, proposer, resp2.Txs)
	applyJoinTxs(t, replica, resp2.Txs)
	pb2, rb2 := bitCount(proposer), bitCount(replica)
	if pb2 != pb-1 || rb2 != rb-1 {
		t.Fatalf("after LEAV bits proposer=%d replica=%d want both %d (drop by 1)", pb2, rb2, pb-1)
	}
}

func lnprHasSubject(blob types.LNPRBlob, subj []byte) bool {
	for _, s := range blob.Subjects {
		if bytes.Equal(s.Subject, subj) && s.Weight > 0 {
			return true
		}
	}
	return false
}

func readLastPrepare(t *testing.T, dir string) lastPrepareFile {
	t.Helper()
	bz, err := os.ReadFile(filepath.Join(dir, "last-prepare"))
	if err != nil {
		t.Fatalf("last-prepare: %v", err)
	}
	var rec lastPrepareFile
	if err := json.Unmarshal(bz, &rec); err != nil {
		t.Fatalf("last-prepare json %q: %v", bz, err)
	}
	return rec
}
