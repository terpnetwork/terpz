package keeper

import (
	"bytes"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func stallJoinDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LEANVAL_MEMBERSHIP_DISABLE", "")
	t.Setenv("LEANVAL_MEMBERSHIP_DIR", dir)
	return dir
}

func stallGenesisTwo(t *testing.T, ks ...*Keeper) (g0, g1, j0, j1 []byte) {
	t.Helper()
	g0 = bytes.Repeat([]byte{0xa1}, 32)
	g1 = bytes.Repeat([]byte{0xa2}, 32)
	j0 = bytes.Repeat([]byte{0xb1}, 32)
	j1 = bytes.Repeat([]byte{0xb2}, 32)
	for _, k := range ks {
		k.AllowDummy = true
		k.AcceptProof(0, g0, 10)
		k.AcceptProof(0, g1, 10)
		if n := bitCount(k); n != 2 {
			t.Fatalf("genesis bits=%d want 2", n)
		}
	}
	return g0, g1, j0, j1
}

func brokenFoldProof(roots []byte) []byte {
	p := make([]byte, 10+types.ObjectRootsSize)
	copy(p[0:4], []byte("STWO"))
	p[4] = 2
	p[5] = 5
	copy(p[6:10], []byte("FOLD"))
	copy(p[10:], padRoots(roots))
	p[10] ^= 0xff
	return p
}

func processStatus(t *testing.T, k *Keeper, height int64, txs [][]byte) abci.ResponseProcessProposal_ProposalStatus {
	t.Helper()
	resp, err := k.WrapProcessProposal(nil)(&abci.RequestProcessProposal{Height: height, Txs: txs})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("nil Process response")
	}
	t.Logf("Process %s height=%d txs=%d", resp.Status.String(), height, len(txs))
	return resp.Status
}

// Replica Process REJECT on LNPR that fails Verify: extra JOIN subject
// with empty or bitflipped Dummy is unprovable.
func TestProcessRejectsUnprovableExtraSubjectsStall(t *testing.T) {
	_ = stallJoinDir(t)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	g0, g1, j0, _ := stallGenesisTwo(t, k)
	roots := k.LastObjectRoots()
	period := types.PeriodFromHeight(4)
	blob := types.LNPRBlob{
		Period: period,
		Subjects: []types.SubjectProof{
			{Subject: g0, Weight: 10, Proof: DummyStwoProveBoundRoots(period, g0, 10, roots)},
			{Subject: g1, Weight: 10, Proof: DummyStwoProveBoundRoots(period, g1, 10, roots)},
			{Subject: j0, Weight: 10, Proof: nil},
		},
	}
	if err := k.VerifyLNPR(blob); err == nil {
		t.Fatal("empty proof on extra JOIN subject must fail Verify")
	}
	if processStatus(t, k, 4, [][]byte{types.EncodeLNPR(blob)}) != abci.ResponseProcessProposal_REJECT {
		t.Fatal("Process must REJECT unprovable extra subject")
	}
	if n := bitCount(k); n != 2 {
		t.Fatalf("REJECT must not commit: bits=%d want 2", n)
	}

	bad := DummyStwoProveBoundRoots(period, j0, 10, roots)
	bad[len(bad)-1] ^= 1
	blob.Subjects[2].Proof = bad
	if err := k.VerifyLNPR(blob); err == nil {
		t.Fatal("bitflipped Dummy on extra JOIN subject must fail Verify")
	}
	if processStatus(t, k, 4, [][]byte{types.EncodeLNPR(blob)}) != abci.ResponseProcessProposal_REJECT {
		t.Fatal("Process must REJECT broken extra Dummy")
	}
	if n := bitCount(k); n != 2 {
		t.Fatalf("REJECT must not commit: bits=%d want 2", n)
	}
}

// Broken fold fails Verify even if Dummy extras would otherwise pass.
func TestProcessRejectsBrokenFoldStall(t *testing.T) {
	_ = stallJoinDir(t)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	g0, g1, j0, _ := stallGenesisTwo(t, k)
	roots := k.LastObjectRoots()
	period := types.PeriodFromHeight(4)
	blob := types.LNPRBlob{
		Period: period,
		Subjects: []types.SubjectProof{
			{Subject: g0, Weight: 10, Proof: brokenFoldProof(roots)},
			{Subject: g1, Weight: 10, Proof: DummyStwoProveBoundRoots(period, g1, 10, roots)},
			{Subject: j0, Weight: 10, Proof: DummyStwoProveBoundRoots(period, j0, 10, roots)},
		},
	}
	if err := k.VerifyLNPR(blob); err == nil {
		t.Fatal("broken fold must fail Verify")
	}
	if processStatus(t, k, 4, [][]byte{types.EncodeLNPR(blob)}) != abci.ResponseProcessProposal_REJECT {
		t.Fatal("Process must REJECT broken fold")
	}
	if n := bitCount(k); n != 2 {
		t.Fatalf("REJECT must not commit: bits=%d want 2", n)
	}
}

// Two keepers: unverifiable proposer LNPR → replica REJECT (expected BFT).
// Proposal does not commit; bits stay 2.
func TestTwoKeepersUnverifiableProposerLNPRReplicaRejectStall(t *testing.T) {
	_ = stallJoinDir(t)
	proposer := NewKeeper(NewMemStore(), DummyStwoGo{})
	replica := NewKeeper(NewMemStore(), DummyStwoGo{})
	_, _, j0, j1 := stallGenesisTwo(t, proposer, replica)

	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10}))
	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10}))

	resp, err := proposer.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(blob.Subjects) < 4 {
		t.Fatalf("proposer must not fall back to 2-subject LNPR when JOIN files exist: subjects=%d", len(blob.Subjects))
	}

	// Corrupt extra JOIN proofs: roster still carries JOIN, but Verify fails.
	for i := range blob.Subjects {
		if bytes.Equal(blob.Subjects[i].Subject, j0) || bytes.Equal(blob.Subjects[i].Subject, j1) {
			if len(blob.Subjects[i].Proof) == 0 {
				blob.Subjects[i].Proof = []byte("bad")
			} else {
				blob.Subjects[i].Proof[len(blob.Subjects[i].Proof)-1] ^= 1
			}
		}
	}
	bad := types.EncodeLNPR(blob)
	if processStatus(t, replica, 4, [][]byte{bad}) != abci.ResponseProcessProposal_REJECT {
		t.Fatal("replica must REJECT unverifiable proposer LNPR (expected BFT stall)")
	}
	if err := replica.ProcessInjectedLNPR([][]byte{bad}); err == nil {
		t.Fatal("ApplyLNPR must not commit unverifiable LNPR")
	}
	if pb, rb := bitCount(proposer), bitCount(replica); pb != 2 || rb != 2 {
		t.Fatalf("proposal must not commit: proposer=%d replica=%d want 2,2", pb, rb)
	}
}

// JOIN files present: Prepare still emits JOIN subjects even if Dummy is
// closed (unverifiable). Genesis-only (2 subjects) would hide the stall.
func TestRosterDoesNotFallBackWhenJoinFilesExistStall(t *testing.T) {
	_ = stallJoinDir(t)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	_, _, j0, j1 := stallGenesisTwo(t, k)
	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10}))
	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10}))

	k.AllowDummy = false
	resp, err := k.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(blob.Subjects) < 4 {
		t.Fatalf("JOIN files present: subjects=%d — do not hide stall with genesis-only roster", len(blob.Subjects))
	}
	if !lnprHasSubject(blob, j0) || !lnprHasSubject(blob, j1) {
		t.Fatalf("JOIN file subjects missing from roster: %+v", subjectsBrief(blob))
	}
	if processStatus(t, k, 4, resp.Txs) != abci.ResponseProcessProposal_REJECT {
		t.Fatal("unverifiable JOIN roster must REJECT (stall), not commit the old 2-bit set")
	}
	if n := bitCount(k); n != 2 {
		t.Fatalf("stall REJECT must leave bits=%d at 2", n)
	}
}

// Honest path: Dummy-proven JOIN subjects ACCEPT and bits grow on both keepers.
func TestJoinHonestLNPRProcessAcceptBitsGrow(t *testing.T) {
	_ = stallJoinDir(t)
	proposer := NewKeeper(NewMemStore(), DummyStwoGo{})
	replica := NewKeeper(NewMemStore(), DummyStwoGo{})
	_, _, j0, j1 := stallGenesisTwo(t, proposer, replica)
	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10}))
	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j1, Weight: 10}))

	resp, err := proposer.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	blob, _, ok := FindLNPR(resp.Txs)
	if !ok {
		t.Fatal("LNPR missing")
	}
	if len(blob.Subjects) < 4 {
		t.Fatalf("honest JOIN LNPR subjects=%d want >=4", len(blob.Subjects))
	}
	if st := processStatus(t, replica, 4, resp.Txs); st != abci.ResponseProcessProposal_ACCEPT {
		t.Fatalf("verifiable JOIN LNPR must ACCEPT, got %s", st)
	}
	if st := processStatus(t, proposer, 4, resp.Txs); st != abci.ResponseProcessProposal_ACCEPT {
		t.Fatalf("proposer Process of honest JOIN LNPR must ACCEPT, got %s", st)
	}
	t.Logf("honest JOIN LNPR ACCEPT subjects=%d", len(blob.Subjects))
	if err := proposer.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatalf("proposer ApplyLNPR: %v", err)
	}
	if err := replica.ProcessInjectedLNPR(resp.Txs); err != nil {
		t.Fatalf("replica ApplyLNPR: %v", err)
	}
	if pb, rb := bitCount(proposer), bitCount(replica); pb < 4 || rb < 4 {
		t.Fatalf("honest path bits proposer=%d replica=%d want both >=4", pb, rb)
	}
}
