package leanval_test

import (
	"context"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval"
	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

type p0FakeStake struct{ n int }

func (f *p0FakeStake) EndBlock(context.Context) ([]abci.ValidatorUpdate, error) {
	f.n++
	return []abci.ValidatorUpdate{{Power: 99}}, nil
}

func TestP0_StakingEndBlockSplit_UnbondingStillRuns(t *testing.T) {
	// SDK TestUnbondingCanComplete: wrap always runs staking EndBlock; drops updates if flag on.
	k := keeper.NewKeeper(nil, nil)
	k.SetOwnsValset(true)
	st := &p0FakeStake{}
	fn := leanval.WrapStakingEndBlock(st, k)
	u, err := fn(context.Background())
	if err != nil || st.n != 1 || len(u) != 0 {
		t.Fatalf("unbonding path must run, updates dropped: u=%v n=%d err=%v", u, st.n, err)
	}
}
