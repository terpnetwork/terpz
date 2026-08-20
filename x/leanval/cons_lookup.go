package leanval

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

// ConsLookup is x/staking ValidatorByConsAddr. Lean JOIN subjects are in
// BondedSet / Comet but not in staking; callers must skip ErrNoValidatorFound.
type ConsLookup interface {
	ValidatorByConsAddr(ctx context.Context, addr sdk.ConsAddress) (stakingtypes.ValidatorI, error)
}

type skipUnknownAlloc struct {
	inner keeper.TokenAllocator
	cons  ConsLookup
}

func (s skipUnknownAlloc) AllocateTokens(ctx context.Context, _ int64, votes []abci.VoteInfo) error {
	if s.inner == nil {
		return nil
	}
	kept := make([]abci.VoteInfo, 0, len(votes))
	var tot int64
	for _, v := range votes {
		if s.cons != nil {
			if _, err := s.cons.ValidatorByConsAddr(ctx, sdk.ConsAddress(v.Validator.Address)); err != nil {
				continue
			}
		}
		kept = append(kept, v)
		tot += v.Validator.Power
	}
	return s.inner.AllocateTokens(ctx, tot, kept)
}
