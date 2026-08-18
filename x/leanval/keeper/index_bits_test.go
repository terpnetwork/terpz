package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// loop_id=index-and-bits / join-bit / fold-pi / vp-from-bits
// SOURCES.md Phase 1A: consensus row is deposit-tree INDEX (~5 B), not 32-byte ed25519.
// SOURCES.md Phase 1B: participation is a BITFIELD (OR-merge); PIs = bitfield + state roots
// (JiangXb-son). A pubkey+weight roster in LNPR is not the membership object.

func testPub(b byte) []byte {
	p := bytes.Repeat([]byte{b}, 32)
	return p
}

func storedBitfield(k *Keeper) []byte {
	return k.Store().Get(types.BitfieldKey())
}

func storedIndex(k *Keeper, subject []byte) []byte {
	return k.Store().Get(types.DepositIndexKey(subject))
}

func queryBit(bf []byte, i uint) bool {
	if int(i/8) >= len(bf) {
		return false
	}
	return bf[i/8]&(1<<(i%8)) != 0
}

func bitfieldRoot(bf []byte) []byte {
	h := sha256.Sum256(bf)
	return h[:]
}

func TestIndexAndBits_GenesisSubjectsAreIndicesWithBitsSet(t *testing.T) {
	a, b := testPub(0x01), testPub(0x02)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.InitGenesis(types.GenesisState{
		OwnsValset: true,
		GenesisSubjects: []types.GenesisSubject{
			{PubKey: a, Weight: 10},
			{PubKey: b, Weight: 20},
		},
	})

	idxA := storedIndex(k, a)
	idxB := storedIndex(k, b)
	if len(idxA) < 5 || len(idxB) < 5 {
		t.Fatalf("SOURCES 1A: store must have DepositIndex (>=5 B) per subject, got %d and %d", len(idxA), len(idxB))
	}
	if bytes.Equal(idxA, a) || bytes.Equal(idxB, b) {
		t.Fatal("DepositIndex must not be the 32-byte ed25519 subject")
	}
	ia, ib := types.GetU32(idxA[len(idxA)-4:]), types.GetU32(idxB[len(idxB)-4:])
	if ia == ib {
		t.Fatal("genesis subjects must get distinct indices 0..n-1")
	}
	if (ia != 0 && ia != 1) || (ib != 0 && ib != 1) {
		t.Fatalf("genesis indices want 0 and 1, got %d %d", ia, ib)
	}

	bf := storedBitfield(k)
	if len(bf) == 0 {
		t.Fatal("SOURCES 1B: store must have Bitfield bytes")
	}
	if !queryBit(bf, 0) || !queryBit(bf, 1) {
		t.Fatalf("query bit i: genesis indices 0..n-1 must be set, bf=%x", bf)
	}
	if queryBit(bf, 2) {
		t.Fatal("bit 2 must be clear at genesis of 2 subjects")
	}
}

func TestIndexAndBits_PubkeyListLNPRIsNotBitfield(t *testing.T) {
	a := testPub(0xaa)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.InitGenesis(types.GenesisState{
		OwnsValset:      true,
		GenesisSubjects: []types.GenesisSubject{{PubKey: a, Weight: 10}},
	})
	// Pretend membership is an LNPR subject list (the failed roster).
	blob := types.LNPRBlob{Period: 0, Subjects: []types.SubjectProof{{
		Subject: a, Weight: 10,
		Proof: DummyStwoProveBoundRoots(0, a, 10, nil),
	}}}
	if err := k.ApplyLNPR(blob); err != nil {
		t.Logf("ApplyLNPR: %v", err)
	}
	if len(storedBitfield(k)) == 0 {
		t.Fatal("pubkey-list LNPR is not a bitfield; store Bitfield must exist independently of LNPR subjects")
	}
	if len(storedIndex(k, a)) < 5 {
		t.Fatal("membership SoT is deposit index + bit, not LNPR Dummy (a,b) pairs")
	}
}

func TestJoinBit_AllocNextIndexSetsBitAndLNPRCannotWipe(t *testing.T) {
	stay := testPub(0xaa)
	join := testPub(0xbb)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.InitGenesis(types.GenesisState{
		OwnsValset:      true,
		GenesisSubjects: []types.GenesisSubject{{PubKey: stay, Weight: 10}},
	})
	before := len(k.QueryBondedSet(0))
	if err := k.ApplyJoin(types.JoinBlob{Period: 0, Subject: join, Weight: 8}); err != nil {
		t.Fatal(err)
	}
	idx := storedIndex(k, join)
	if len(idx) < 5 {
		t.Fatal("JOIN = allocate next deposit-tree index (SOURCES 1A), not CheckTx mempool hope")
	}
	next := types.GetU32(idx[len(idx)-4:])
	if next != 1 {
		t.Fatalf("first join after 1 genesis subject must be index 1, got %d", next)
	}
	bf := storedBitfield(k)
	if !queryBit(bf, uint(next)) {
		t.Fatalf("JOIN must set bit %d; bf=%x", next, bf)
	}
	after := k.QueryBondedSet(0)
	if len(after) <= before {
		t.Fatalf("BondedSet debug view must grow: before=%d after=%d", before, len(after))
	}

	// Next LNPR listing only genesis must not clear the join bit (FRICTION: roster wipe).
	if err := k.ApplyLNPR(types.LNPRBlob{Period: 0, Subjects: []types.SubjectProof{{
		Subject: stay, Weight: 10,
		Proof: DummyStwoProveBoundRoots(0, stay, 10, nil),
	}}}); err != nil {
		t.Logf("ApplyLNPR: %v", err)
	}
	bf2 := storedBitfield(k)
	if !queryBit(bf2, uint(next)) {
		t.Fatal("next LNPR cannot wipe the join bit — bit flip is app-state, not flood mempool")
	}
}

func TestFoldPI_MustBindBitfieldRoot_DummyNAndCostumeFail(t *testing.T) {
	a, b := testPub(0x11), testPub(0x22)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	k.InitGenesis(types.GenesisState{
		OwnsValset: true,
		GenesisSubjects: []types.GenesisSubject{
			{PubKey: a, Weight: 3},
			{PubKey: b, Weight: 5},
		},
	})
	bf := storedBitfield(k)
	if len(bf) == 0 {
		t.Fatal("fold-pi requires a stored bitfield (Jiang: PIs = bitfields + state roots)")
	}
	root := bitfieldRoot(bf)
	pairs := foldPairStrings(0, []types.SubjectProof{
		{Subject: a, Weight: 3},
		{Subject: b, Weight: 5},
	}, k.LastObjectRoots())
	joined := strings.Join(pairs, "|")
	if !strings.Contains(joined, hex.EncodeToString(root)) && !bytesContainRoot(pairs, root) {
		t.Fatalf("VerifySameStatementFold instances must bind bitfield root %x; got pairs=%v (not c=3a+5b+7 over subjects)", root, pairs)
	}

	dummy := []byte("DSTW")
	if err := VerifySameStatementFold(dummy, pairs); err == nil {
		t.Fatal("Dummy-N must FAIL")
	}
	// Costume checksum over subjects is not a fold statement.
	costume := append([]byte("STWO"), 2, 5)
	costume = append(costume, []byte("FOLD")...)
	costume = append(costume, []byte("c=3a+5b+7")...)
	if err := VerifySameStatementFold(costume, []string{"3,5", "5,7"}); err == nil {
		t.Fatal("c=3a+5b+7 over subjects must FAIL")
	}
}

func bytesContainRoot(pairs []string, root []byte) bool {
	want := hex.EncodeToString(root)
	for _, p := range pairs {
		if strings.Contains(p, want) || strings.Contains(strings.ToLower(p), want) {
			return true
		}
	}
	return false
}

func TestVpFromBits_OnlySetBits_PowerFromEBNotStakingShares(t *testing.T) {
	on := testPub(0xa1)
	off := testPub(0xa2)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.SetOwnsValset(true)
	// Staking-share costume: huge weights that must NOT become Comet power.
	k.AcceptProof(0, on, 999)
	k.AcceptProof(0, off, 888)
	// Separate EB tree: 1 byte. SOURCES 1B: 1 B EB + 5 B index.
	k.Store().Set(types.DepositIndexKey(on), types.DepositIndexBytes(0))
	k.Store().Set(types.DepositIndexKey(off), types.DepositIndexBytes(1))
	k.Store().Set(types.EBKey(0), []byte{16}) // fp8-ish min 16
	k.Store().Set(types.EBKey(1), []byte{32})
	// Only bit 0 set — off validator must not get a VP update with power>0.
	k.Store().Set(types.BitfieldKey(), []byte{0b00000001})

	ups := k.ValidatorUpdates(0)
	var sawOn, sawOff bool
	for _, u := range ups {
		ed := u.PubKey.GetEd25519()
		if bytes.Equal(ed, on) {
			sawOn = true
			if u.Power != 16 {
				t.Fatalf("power must come from EB byte 16, not staking/BondedSet 999; got %d", u.Power)
			}
		}
		if bytes.Equal(ed, off) && u.Power != 0 {
			sawOff = true
			t.Fatalf("bit 1 is 0: no positive ValidatorUpdate, got power=%d (do not wrap x/staking)", u.Power)
		}
	}
	if !sawOn {
		t.Fatal("ValidatorUpdates only for bits that are 1: missing index 0")
	}
	_ = sawOff
}

func TestQueryBitI(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.InitGenesis(types.GenesisState{
		OwnsValset: true,
		GenesisSubjects: []types.GenesisSubject{
			{PubKey: testPub(1), Weight: 1},
			{PubKey: testPub(2), Weight: 1},
			{PubKey: testPub(3), Weight: 1},
		},
	})
	bf := storedBitfield(k)
	for i := uint(0); i < 3; i++ {
		if !queryBit(bf, i) {
			t.Fatalf("query bit %d: want 1 (SOURCES 1B participation bitfield)", i)
		}
	}
}

func TestQueryBondedSetIsDebugViewNotRosterSoT(t *testing.T) {
	on := testPub(0x51)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.InitGenesis(types.GenesisState{
		OwnsValset:      true,
		GenesisSubjects: []types.GenesisSubject{{PubKey: on, Weight: 16}},
	})
	k.Store().IteratePrefix([]byte{types.BondedPrefix}, func(key, _ []byte) bool {
		k.Store().Delete(key)
		return true
	})
	if n := len(k.BondedSet(0)); n != 0 {
		t.Fatalf("BondedSet wipe failed: %d", n)
	}
	q := k.QueryBondedSet(0)
	if len(q) != 1 {
		t.Fatalf("debug BondedSet query must rebuild from bits, got %d (roster is not SoT)", len(q))
	}
	if !bytes.Equal(q[0].Subject, on) || q[0].Weight != 16 {
		t.Fatalf("query row want subject+EB, got %+v", q[0])
	}
}
