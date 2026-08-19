package keeper

import (
	"bytes"
	"strings"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestProveVerifySameStatementFold_DummyNFails(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.AcceptProof(0, bytes32(0x11), 10)
	k.AcceptProof(0, bytes32(0x22), 10)
	pairs := foldPairStrings(0, nil, k.LastObjectRoots())
	if len(pairs) < 3 || !isHexRoot(pairs[0]) {
		t.Fatalf("fold PIs must be hex roots, got %v", pairs)
	}

	if err := VerifySameStatementFold([]byte("DSTW"), pairs); err == nil {
		t.Fatal("Dummy-N must FAIL")
	}

	proof, err := ProveSameStatementFold(pairs)
	if err != nil {
		if strings.Contains(err.Error(), "not on PATH") {
			t.Fatalf("lean-stwo-fold must be runnable for this test: %v", err)
		}
		t.Fatal(err)
	}
	if !isFoldProof(proof) {
		t.Fatalf("prove must emit STWO FOLD, got %q…", proof[:min(20, len(proof))])
	}
	if bytes.Contains(proof, []byte("DSTW")) {
		t.Fatal("fold prove emitted Dummy")
	}
	if err := VerifySameStatementFold(proof, pairs); err != nil {
		t.Fatalf("verify of own prove: %v", err)
	}

	// Wrong public inputs must fail even with a well-formed STWO FOLD.
	wrong := append([]string(nil), pairs...)
	wrong[0] = strings.Repeat("11", 32)
	if err := VerifySameStatementFold(proof, wrong); err == nil {
		t.Fatal("verify must bind bitfield root")
	}
}

func TestLeaveThenFoldBindsClearedBitfield(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.ClearPendingMembership()
	g0 := bytes32(0xaa)
	g1 := bytes32(0xbb)
	joiner := bytes32(0xcc)
	k.AcceptProof(0, g0, 10)
	k.AcceptProof(0, g1, 10)
	NoteMembershipBytes(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: joiner, Weight: 10}))
	raw := k.buildLNPR(0)
	blob, ok := types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("join lnpr")
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	if n := countSetBits(k); n != 3 {
		t.Fatalf("after JOIN bits=%d want 3", n)
	}
	beforePairs := foldPairStrings(0, nil, k.LastObjectRoots())

	NoteMembershipBytes(types.EncodeLeave(types.LeaveBlob{Period: 0, Subject: joiner}))
	raw = k.buildLNPR(0)
	blob, ok = types.DecodeLNPR(raw)
	if !ok {
		t.Fatal("leave lnpr")
	}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Fatal(err)
	}
	if n := countSetBits(k); n != 2 {
		t.Fatalf("after LEAV bits=%d want 2", n)
	}
	afterPairs := foldPairStrings(0, nil, k.LastObjectRoots())
	if beforePairs[0] == afterPairs[0] {
		t.Fatal("LEAV must change the bitfield root the fold binds")
	}

	proof, err := ProveSameStatementFold(afterPairs)
	if err != nil {
		if strings.Contains(err.Error(), "not on PATH") {
			t.Fatalf("lean-stwo-fold must be runnable for this test: %v", err)
		}
		t.Fatal(err)
	}
	if err := VerifySameStatementFold(proof, afterPairs); err != nil {
		t.Fatalf("post-LEAV fold verify: %v", err)
	}
	if err := VerifySameStatementFold(proof, beforePairs); err == nil {
		t.Fatal("post-LEAV fold must not verify against the pre-LEAV bitfield root")
	}
}
