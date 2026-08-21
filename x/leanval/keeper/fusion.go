package keeper

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TokenAllocator is x/distribution AllocateTokens (F1). We reuse it so
// commission + delegator shares stay stock; only the *weights* are Lean.
type TokenAllocator interface {
	AllocateTokens(ctx context.Context, totalPreviousPower int64, bondedVotes []abci.VoteInfo) error
}

// VoteInfosFromBondedSet maps proven EB to VoteInfo using cons-addr from
// the ed25519 subject pubkey (same encoding as ValidatorUpdates).
func VoteInfosFromBondedSet(set []SubjectPower) (votes []abci.VoteInfo, total int64) {
	for _, s := range set {
		if s.Weight <= 0 || !s.HasProof {
			continue
		}
		addr := consAddrFromSubject(s.Subject)
		if len(addr) == 0 {
			continue
		}
		votes = append(votes, abci.VoteInfo{
			Validator: abci.Validator{
				Address: addr,
				Power:   s.Weight,
			},
			BlockIdFlag: cmtproto.BlockIDFlagCommit,
		})
		total += s.Weight
	}
	return votes, total
}

func consAddrFromSubject(pub []byte) []byte {
	if len(pub) == 0 {
		return nil
	}
	return ed25519.PubKey(pub).Address()
}

// AllocateDelegatorFees runs F1 on last period BondedSet (not VoteInfos).
// Missing/late proofs (weight 0) get no share — they also cannot attest.
func (k *Keeper) AllocateDelegatorFees(ctx sdk.Context, alloc TokenAllocator) error {
	if alloc == nil {
		return nil
	}
	votes := ctx.VoteInfos()
	if len(votes) == 0 {
		// No certificate → no VoteInfos. Do not forge BlockIdFlagCommit.
		return nil
	}
	var total int64
	out := make([]abci.VoteInfo, 0, len(votes))
	for _, v := range votes {
		if v.BlockIdFlag != cmtproto.BlockIDFlagCommit {
			continue
		}
		total += v.Validator.Power
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return alloc.AllocateTokens(ctx, total, out)
}
