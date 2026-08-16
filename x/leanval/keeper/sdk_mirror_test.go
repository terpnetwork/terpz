package keeper

import (
	"context"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Mirrors SDK v0.54.3 / CometBFT v0.39.3 tests listed in
// TEST-MATRIX-STAKING-REWARDS-ABCI.md. Skips name the SDK TestFunc when
// staking/distr/slashing keepers are not wired.

func TestLean_DelegateDoesNotChangeVPWhenOwnsValset(t *testing.T) {
	t.Skip("SDK TestDelegation / TestMsgDelegate: no staking keeper in leanval unit fixture — tokens stay F1; BondedSet must not change on MsgDelegate")
}

func TestLean_RedelegateDoesNotChangeBondedSet(t *testing.T) {
	t.Skip("SDK TestRedelegation / TestMsgBeginRedelegate: no staking keeper — shares move F1; Lean EB unchanged")
}

func TestLean_UnbondingMaturityDoesNotTouchBondedSet(t *testing.T) {
	t.Skip("SDK TestUnbondingCanComplete / TestUnbondingDelegation: no staking keeper — maturity pays tokens, not LastPowerKey")
}

func TestLean_CreateValidatorDoesNotEmitBondedSet(t *testing.T) {
	t.Skip("SDK TestMsgCreateValidator: no staking keeper — create stores tokens; Comet VP only after accepted proof when owns_valset")
}

func TestLean_JailDoesNotAutoZeroBondedSet(t *testing.T) {
	t.Skip("SDK TestRevocation / TestUndelegateSelfDelegationBelowMinSelfDelegation: no staking keeper — jail is stock; BondedSet not inferred from jailed")
}

func TestLean_TombstoneUnjailStillStock(t *testing.T) {
	t.Skip("SDK TestUnjail: no slashing keeper — tombstone stays x/slashing")
}

func TestLean_TokensToConsensusPowerIgnoredWhenOwnsValset(t *testing.T) {
	t.Skip("SDK TestTokensToConsensusPower: no staking keeper — consensus power is proven EB, not TokensToConsensusPower")
}

func TestLean_StakingPowerIndexNotLastValidatorPower(t *testing.T) {
	t.Skip("SDK TestUpdateValidatorByPowerIndex / TestApplyAndReturnValidatorSetUpdatesPowerDecrease: staking power index kept; Comet updates remapped to BondedSet")
}

func TestLean_InstantSlashStillAllowed(t *testing.T) {
	t.Skip("SDK TestJailAndSlash: no slashing keeper — instant slash (EB=0) remains allowed outside STARK")
}

func TestLean_WithdrawRewardsStayF1(t *testing.T) {
	t.Skip("SDK TestWithdrawDelegationRewardsBasic: no distribution keeper — withdraw path stays F1")
}

func TestLean_AllocateTokensUsesBondedSetNotVoteInfos(t *testing.T) {
	// SDK TestAllocateTokensToManyValidators / TestBeginBlockToMultipleValidators
	// + Comet TestFinalizeBlockDecidedLastCommit: fees use BondedSet, not last-commit VoteInfos.
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
	// allocation.go: if totalPreviousPower == 0 { community pool }.
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
	// SDK TestBeginBlockNoOp / TestBeginBlockToMultipleValidators remapped.
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
	// Contrast SDK TestValidatorMissedBlockBitmap_SmallWindow / TestJailAndSlash.
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
	// Comet TestProcessProposal + TestDummyStwoValidAndBitflip.
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	proof := DummyStwoProve(3, 4)
	proof[14] ^= 1
	blob := types.EncodeLNPR(types.LNPRBlob{
		Period: types.PeriodFromHeight(1),
		Subjects: []types.SubjectProof{{
			Subject: []byte("v-bitflip"),
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
	// SDK TestApplyAndReturnValidatorSetUpdatesPowerDecrease +
	// Comet TestFinalizeBlockValidatorUpdates.
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	k.SetEndPeriod(1)
	subj := make([]byte, 32)
	subj[1] = 7
	k.AcceptProof(1, subj, 21)
	ups, err := k.EndBlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
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
