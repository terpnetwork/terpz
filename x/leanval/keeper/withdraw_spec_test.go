package keeper

import (
	"bytes"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestWithdraw_DummyDSTWIsNotAProof(t *testing.T) {
	err := VerifyNoWithdrawProof([]byte("DSTWDSTW"), 1, make([]byte, 32), make([]byte, 32))
	if err == nil {
		t.Fatal("Dummy DSTW must not verify as no-withdraw")
	}
	err = VerifyPartialWithdrawProof([]byte("DSTWDSTW"), 1, make([]byte, 32), make([]byte, 32))
	if err == nil {
		t.Fatal("Dummy DSTW must not verify as partial withdraw")
	}
}

func TestWithdraw_CommitIsHashNotAddress(t *testing.T) {
	addr := bytes.Repeat([]byte{0xaa}, 20)
	secret := bytes.Repeat([]byte{0xbb}, 32)
	c := WithdrawCommit(addr, secret)
	if bytes.Equal(c[:20], addr) {
		t.Fatal("commitment must not be the withdrawal address")
	}
	if bytes.Equal(c, secret) {
		t.Fatal("commitment must not be the secret")
	}
	c2 := WithdrawCommit(addr, bytes.Repeat([]byte{0xcc}, 32))
	if bytes.Equal(c, c2) {
		t.Fatal("different secret must change H(addr, secret)")
	}
}

func TestWithdraw_AccumulatorDoesNotReplaceBondedSetPower(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	subj := []byte("join-leav-subject-32-bytes!!!!!")
	k.AcceptProof(3, subj, 100)
	commit := WithdrawCommit([]byte("addr"), []byte("secret"))
	k.PutWithdrawCommit(3, commit)
	k.PutNoWithdrawAcc(3, NoWithdrawAcc(3, make([]byte, 32), commit))
	set := k.BondedSet(3)
	if len(set) != 1 || set[0].Weight != 100 {
		t.Fatalf("BondedSet remains power SoT, got %+v", set)
	}
	if bytes.Equal(k.GetWithdrawCommit(3), subj) {
		t.Fatal("withdraw commit is not the LNPR subject")
	}
}

func TestWithdraw_JoinLeaveRemainLNPRSubjects(t *testing.T) {
	join := types.EncodeJoin(types.JoinBlob{Period: 1, Subject: []byte("alice"), Weight: 10})
	leav := types.EncodeLeave(types.LeaveBlob{Period: 1, Subject: []byte("alice")})
	if !types.HasPrefix(join, types.PrefixJOIN) || !types.HasPrefix(leav, types.PrefixLEAV) {
		t.Fatal("JOIN/LEAV prefixes")
	}
}

func TestWithdraw_PartialIsSeparateKind(t *testing.T) {
	if withdrawBin() == "" {
		t.Skip("lean-withdraw not built")
	}
	commit := WithdrawCommit(bytes.Repeat([]byte{0x11}, 20), bytes.Repeat([]byte{0x22}, 32))
	prev := make([]byte, 32)
	acc, nw, err := ProveNoWithdraw(4, prev, commit)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyNoWithdrawProof(nw, 4, acc, prev); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPartialWithdrawProof(nw, 4, commit, acc); err == nil {
		t.Fatal("NWDA must not verify as PWDW")
	}
	next, pw, err := ProvePartialWithdraw(4, commit, 9)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPartialWithdrawProof(pw, 4, commit, next); err != nil {
		t.Fatal(err)
	}
	if err := VerifyNoWithdrawProof(pw, 4, acc, prev); err == nil {
		t.Fatal("PWDW must not verify as NWDA")
	}
	if !bytes.Equal(next, PartialNewCommit(4, commit, 9)) {
		t.Fatal("partial new commit mismatch")
	}
}

func TestWithdraw_StwoProveVerifyNoWithdraw(t *testing.T) {
	if withdrawBin() == "" {
		t.Skip("lean-withdraw not built")
	}
	commit := WithdrawCommit(bytes.Repeat([]byte{0xab}, 20), bytes.Repeat([]byte{0xcd}, 32))
	prev := make([]byte, 32)
	acc, proof, err := ProveNoWithdraw(7, prev, commit)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyNoWithdrawProof(proof, 7, acc, prev); err != nil {
		t.Fatal(err)
	}
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.PutNoWithdrawAcc(7, acc)
	if !bytes.Equal(k.GetNoWithdrawAcc(7), acc) {
		t.Fatal("accumulator miss")
	}
}
