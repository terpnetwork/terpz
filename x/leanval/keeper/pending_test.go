package keeper

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestApplyLNPRSameBlockTwoJoins(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	a := make([]byte, 32)
	a[0] = 1
	b := make([]byte, 32)
	b[0] = 2
	c := make([]byte, 32)
	c[0] = 3
	k.AcceptProof(0, a, 10)
	blob := types.LNPRBlob{
		Period: 0,
		Subjects: []types.SubjectProof{
			{Subject: a, Weight: 10, Proof: DummyStwoProveBound(0, a, 10)},
			{Subject: b, Weight: 10, Proof: DummyStwoProveBound(0, b, 10)},
			{Subject: c, Weight: 10, Proof: DummyStwoProveBound(0, c, 10)},
		},
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	set := k.BondedSet(0)
	if len(set) != 3 {
		t.Fatalf("same-block two joins: got %d %+v", len(set), set)
	}
}

func TestApplyLNPRLeaveDropsAndZeros(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	stay := make([]byte, 32)
	stay[0] = 1
	gone := make([]byte, 32)
	gone[0] = 2
	k.AcceptProof(0, stay, 10)
	k.AcceptProof(0, gone, 10)
	_ = k.ValidatorUpdates(0)
	blob := types.LNPRBlob{
		Period: 0,
		Subjects: []types.SubjectProof{
			{Subject: stay, Weight: 10, Proof: DummyStwoProveBound(0, stay, 10)},
		},
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	set := k.BondedSet(0)
	if len(set) != 1 || set[0].Subject[0] != 1 {
		t.Fatalf("leave must drop: %+v", set)
	}
	ups := k.ValidatorUpdates(0)
	var zeroed bool
	for _, u := range ups {
		if u.Power == 0 {
			zeroed = true
		}
	}
	if !zeroed {
		t.Fatalf("leave must emit power 0: %+v", ups)
	}
}

func TestAllocateSkipsLeftValidator(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	stay := make([]byte, 32)
	stay[0] = 1
	gone := make([]byte, 32)
	gone[0] = 2
	k.AcceptProof(0, stay, 10)
	k.AcceptProof(0, gone, 10)
	if err := k.ApplyLNPR(types.LNPRBlob{
		Period: 0,
		Subjects: []types.SubjectProof{
			{Subject: stay, Weight: 10, Proof: DummyStwoProveBound(0, stay, 10)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	votes, total := VoteInfosFromBondedSet(k.BondedSet(0))
	if total != 10 || len(votes) != 1 {
		t.Fatalf("left val must not get F1 weight: votes=%d total=%d", len(votes), total)
	}
}

func TestMergePendingJoinAndLeave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lean-pending.json")
	stay := make([]byte, 32)
	stay[0] = 0xaa
	join := make([]byte, 32)
	join[0] = 0xbb
	leave := make([]byte, 32)
	leave[0] = 0xcc
	body := []byte(`{"join":[{"pubkey":"` + hex.EncodeToString(join) + `","weight":7}],"leave":["` + hex.EncodeToString(leave) + `"]}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEANVAL_PENDING", path)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	set := []SubjectPower{
		{Subject: stay, Weight: 10, HasProof: true},
		{Subject: leave, Weight: 10, HasProof: true},
	}
	subs := k.mergePending(0, set)
	if len(subs) != 2 {
		t.Fatalf("want stay+join: %+v", subs)
	}
	var sawJoin, sawLeave bool
	for _, s := range subs {
		if s.Subject[0] == 0xbb && s.Weight == 7 {
			sawJoin = true
		}
		if s.Subject[0] == 0xcc {
			sawLeave = true
		}
	}
	if !sawJoin || sawLeave {
		t.Fatalf("pending merge: %+v", subs)
	}
}
