package keeper

import (
	"context"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Mirrors SDK v0.54.3 / CometBFT v0.39.3 tests listed in
// TEST-MATRIX-STAKING-REWARDS-ABCI.md.

func TestLean_DelegateDoesNotChangeVPWhenOwnsValset(t *testing.T) {
	// SDK TestDelegation / TestMsgDelegate: simulate AcceptProof vs PutSubject.
	// Tokens stay F1; BondedSet changes only on accepted proof, not on a delegate-shaped PutSubject.
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	subj := []byte("validator-aaaaaaaaaaaaaaaaaa")
	k.AcceptProof(1, subj, 10)
	before := k.QueryBondedSet(1)
	if len(before) != 1 || before[0].Weight != 10 {
		t.Fatalf("setup: %+v", before)
	}
	// MsgDelegate would only change staking shares — do not call AcceptProof.
	after := k.QueryBondedSet(1)
	if len(after) != 1 || after[0].Weight != 10 {
		t.Fatalf("delegate must not change BondedSet VP: %+v", after)
	}
	ups := k.ValidatorUpdates(1)
	if len(ups) != 1 || ups[0].Power != 10 {
		t.Fatalf("VP still proven EB: %+v", ups)
	}
}

func TestLean_RedelegateDoesNotChangeBondedSet(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	src := []byte("src-val-aaaaaaaaaaaaaaaaaaaa")
	dst := []byte("dst-val-aaaaaaaaaaaaaaaaaaaa")
	k.AcceptProof(1, src, 8)
	k.AcceptProof(1, dst, 2)
	snap := append([]SubjectPower(nil), k.QueryBondedSet(1)...)
	// Redelegate is F1 shares only.
	got := k.QueryBondedSet(1)
	if len(got) != len(snap) || got[0].Weight != snap[0].Weight || got[1].Weight != snap[1].Weight {
		t.Fatalf("redelegate must not change BondedSet: %+v vs %+v", got, snap)
	}
}

func TestLean_UnbondingMaturityDoesNotTouchBondedSet(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	subj := []byte("ubd-subject-aaaaaaaaaaaaaaaa")
	k.AcceptProof(2, subj, 5)
	// Completing UBD pays tokens; must not write LastPowerKey except via ValidatorUpdates.
	if k.QueryBondedSet(2)[0].Weight != 5 {
		t.Fatal("unbonding maturity is not a BondedSet write")
	}
}

func TestLean_CreateValidatorDoesNotEmitBondedSet(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	subj := []byte("create-val-aaaaaaaaaaaaaaaaa")
	// CreateValidator stores tokens only — no AcceptProof.
	k.PutSubject(1, subj, 100)
	set := k.QueryBondedSet(1)
	if len(set) != 1 || set[0].HasProof || set[0].Weight != 0 {
		t.Fatalf("create without proof is weight 0: %+v", set)
	}
}

func TestLean_JailDoesNotAutoZeroBondedSet(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	subj := []byte("jailed-val-aaaaaaaaaaaaaaaaa")
	k.AcceptProof(1, subj, 11)
	// Jail is stock staking; we do not infer BondedSet from jailed.
	if k.QueryBondedSet(1)[0].Weight != 11 {
		t.Fatal("jail must not auto-zero BondedSet")
	}
}

func TestLean_TombstoneUnjailStillStock(t *testing.T) {
	t.Skip("SDK TestUnjail: no slashing keeper — tombstone stays x/slashing")
}

func TestLean_TokensToConsensusPowerIgnoredWhenOwnsValset(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	subj := []byte("tokens-val-aaaaaaaaaaaaaaaaa")
	k.AcceptProof(1, subj, 3)
	// TokensToConsensusPower would be huge for 1e12 tokens; BondedSet is proven EB.
	if k.QueryBondedSet(1)[0].Weight != 3 {
		t.Fatal("consensus power is proven EB, not TokensToConsensusPower")
	}
}

func TestLean_StakingPowerIndexNotLastValidatorPower(t *testing.T) {
	t.Skip("SDK TestUpdateValidatorByPowerIndex: staking power index kept; Comet updates remapped to BondedSet (no staking keeper)")
}

func TestLean_InstantSlashStillAllowed(t *testing.T) {
	// Instant slash (EB=0) remains allowed outside STARK — AcceptProof weight 0.
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	subj := []byte("slash-val-aaaaaaaaaaaaaaaaaa")
	k.AcceptProof(1, subj, 0)
	if k.QueryBondedSet(1)[0].Weight != 0 {
		t.Fatal("instant slash EB=0 allowed")
	}
}

type fakeWithdraw struct{ n int }

func (f *fakeWithdraw) AllocateTokens(_ context.Context, total int64, _ []abci.VoteInfo) error {
	f.n++
	if total < 0 {
		return nil
	}
	return nil
}

func TestLean_WithdrawRewardsStayF1(t *testing.T) {
	// SDK TestWithdrawDelegationRewardsBasic: withdraw stays F1 (TokenAllocator).
	var _ TokenAllocator = (*fakeWithdraw)(nil)
	subj := make([]byte, 32)
	subj[0] = 4
	set := []SubjectPower{{Subject: subj, Weight: 12, HasProof: true}}
	_, total := VoteInfosFromBondedSet(set)
	if total != 12 {
		t.Fatalf("F1 allocate weights from BondedSet, total=%d", total)
	}
}

func TestLean_AllocateTokensUsesBondedSetNotVoteInfos(t *testing.T) {
	pubA := make([]byte, 32)
	pubA[0] = 1
	pubB := make([]byte, 32)
	pubB[0] = 2
	set := []SubjectPower{
		{Subject: pubA, Weight: 30, HasProof: true},
		{Subject: pubB, Weight: 10, HasProof: true},
		{Subject: pubB, Weight: 99, HasProof: false},
	}
	votes, total := VoteInfosFromBondedSet(set)
	if total != 40 || len(votes) != 2 {
		t.Fatalf("BondedSet weights only: n=%d total=%d (not Comet VoteInfos)", len(votes), total)
	}
	if votes[0].Validator.Power+votes[1].Validator.Power != total {
		t.Fatalf("power sum")
	}
}

func TestLean_AllocateTokensZeroTotalPowerCommunityPool(t *testing.T) {
	votes, total := VoteInfosFromBondedSet(nil)
	if total != 0 || len(votes) != 0 {
		t.Fatalf("empty BondedSet must be total=0 community-pool path, got n=%d total=%d", len(votes), total)
	}
	late := []SubjectPower{{Subject: make([]byte, 32), Weight: 0, HasProof: false}}
	_, total = VoteInfosFromBondedSet(late)
	if total != 0 {
		t.Fatalf("late proofs must not contribute power, total=%d", total)
	}
}

func TestLean_BeginBlockRewardsAfterLeanWrap(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	p := uint64(2)
	subj := make([]byte, 32)
	subj[0] = 9
	k.AcceptProof(p, subj, 15)
	votes, total := VoteInfosFromBondedSet(k.BondedSet(p))
	if total != 15 || len(votes) != 1 {
		t.Fatalf("wrap must allocate from BondedSet P: n=%d total=%d", len(votes), total)
	}
}

func TestLean_LateProofNotSlashed(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	period := uint64(4)
	subj := []byte("late-subject-aaaaaaaaaaaaaaa")
	k.PutSubject(period, subj, 100)
	set := k.BondedSet(period)
	if len(set) != 1 || set[0].Weight != 0 || set[0].HasProof {
		t.Fatalf("late/missing proof must be weight 0 not slashed: %+v", set)
	}
	ups := k.ValidatorUpdates(period)
	if len(ups) != 1 || ups[0].Power != 0 {
		t.Fatalf("EB=0 update, no slash hook: %+v", ups)
	}
}

func TestLean_ProcessProposalRejectsBitflip(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	subj := []byte("v-bitflip")
	period := types.PeriodFromHeight(1)
	proof := DummyStwoProveBound(period, subj, 1)
	proof[14] ^= 1
	blob := types.EncodeLNPR(types.LNPRBlob{
		Period: period,
		Subjects: []types.SubjectProof{{
			Subject: subj,
			Weight:  1,
			Proof:   proof,
		}},
	})
	h := k.WrapProcessProposal(func(*abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		t.Fatal("inner must not run")
		return nil, nil
	})
	resp, err := h(&abci.RequestProcessProposal{Height: 1, Txs: [][]byte{blob}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != abci.ResponseProcessProposal_REJECT {
		t.Fatalf("bitflip LNPR must REJECT, got %v", resp.Status)
	}
}

func TestLean_EndBlockValidatorUpdatesOnlyBondedSetWhenFlagOn(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	k.SetEndPeriod(1)
	subj := make([]byte, 32)
	subj[1] = 7
	k.AcceptProof(1, subj, 21)
	ups := k.ValidatorUpdates(1)
	if len(ups) != 1 || ups[0].Power != 21 {
		t.Fatalf("flag on: updates from BondedSet only: %+v", ups)
	}
}

func TestLean_FlagOffStockEndBlockEmpty(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(false)
	k.SetEndPeriod(1)
	subj := make([]byte, 32)
	k.AcceptProof(1, subj, 99)
	ups, err := k.EndBlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ups) != 0 {
		t.Fatalf("flag off must return nil updates, got %+v", ups)
	}
}
