package leanval_test

import (
	"context"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval"
	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

type fakeStake struct{ n int }

func (f *fakeStake) EndBlock(context.Context) ([]abci.ValidatorUpdate, error) {
	f.n++
	return []abci.ValidatorUpdate{{Power: 99}}, nil
}

func TestFlagOffDoesNotEmitLeanUpdates(t *testing.T) {
	k := keeper.NewKeeper(nil, nil)
	k.SetOwnsValset(false)
	k.SetPendingUpdates([]abci.ValidatorUpdate{{Power: 7}})
	u, err := k.EndBlock(context.Background())
	if err != nil || len(u) != 0 {
		t.Fatalf("flag off must not emit lean valset: %v %v", u, err)
	}
}

func TestFlagOnEmitsLeanUpdates(t *testing.T) {
	k := keeper.NewKeeper(nil, nil)
	k.SetOwnsValset(true)
	k.SetPendingUpdates([]abci.ValidatorUpdate{{Power: 7}})
	u, err := k.EndBlock(context.Background())
	if err != nil || len(u) != 1 || u[0].Power != 7 {
		t.Fatalf("flag on: %+v %v", u, err)
	}
}

func TestWrapRunsStakingButDropsUpdatesWhenOwns(t *testing.T) {
	k := keeper.NewKeeper(nil, nil)
	k.SetOwnsValset(true)
	st := &fakeStake{}
	fn := leanval.WrapStakingEndBlock(st, k)
	u, err := fn(context.Background())
	if err != nil || len(u) != 0 || st.n != 1 {
		t.Fatalf("staking EndBlock must run (unbonding) but drop val updates: u=%v n=%d err=%v", u, st.n, err)
	}
	if types.FlagOwnsValset != "leanval_owns_valset" {
		t.Fatal(types.FlagOwnsValset)
	}
}

func TestWrapCallsStakingWhenOff(t *testing.T) {
	k := keeper.NewKeeper(nil, nil)
	k.SetOwnsValset(false)
	st := &fakeStake{}
	fn := leanval.WrapStakingEndBlock(st, k)
	u, err := fn(context.Background())
	if err != nil || st.n != 1 || u[0].Power != 99 {
		t.Fatalf("got u=%v n=%d err=%v", u, st.n, err)
	}
}
