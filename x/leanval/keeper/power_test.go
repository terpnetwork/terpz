package keeper

import (
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestBondedSetMissingProofWeightZero(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	period := uint64(3)
	subj := append([]byte("cons-addr-aaaaaaaaaaaaaaaa"), make([]byte, 32)...)[:32]
	k.PutSubject(period, subj, 100) // claimed 100, no accepted proof

	set := k.BondedSet(period)
	if len(set) != 1 {
		t.Fatalf("len=%d", len(set))
	}
	if set[0].Weight != 0 {
		t.Fatalf("missing proof must be weight 0, got %d", set[0].Weight)
	}
	if set[0].HasProof {
		t.Fatal("HasProof should be false")
	}

	ups := k.ValidatorUpdates(period)
	if len(ups) != 1 || ups[0].Power != 0 {
		t.Fatalf("updates from BondedSet only, power 0: %+v", ups)
	}
}

func TestBondedSetAcceptedWeight(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	period := uint64(1)
	subj := []byte{1, 2, 3, 4}
	k.AcceptProof(period, subj, 42)
	set := k.BondedSet(period)
	if len(set) != 1 || set[0].Weight != 42 || !set[0].HasProof {
		t.Fatalf("%+v", set)
	}
}

func TestPeriodStubHour(t *testing.T) {
	if types.BlocksPerPeriod != 600 {
		t.Fatalf("N must be documented 600, got %d", types.BlocksPerPeriod)
	}
	if types.PeriodFromHeight(599) != 0 || types.PeriodFromHeight(600) != 1 {
		t.Fatal("period = height / 600")
	}
}
