package leanval

import (
	"context"

	distr "github.com/cosmos/cosmos-sdk/x/distribution"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

// DistrAppModule skips STF reward allocation when leanval owns the valset.
// Lean Phase 1B: no BeginBlock participation rewards; EB only from proofs.
type DistrAppModule struct {
	distr.AppModule
	lean *keeper.Keeper
}

func WrapDistribution(am distr.AppModule, lean *keeper.Keeper) DistrAppModule {
	return DistrAppModule{AppModule: am, lean: lean}
}

func (am DistrAppModule) BeginBlock(ctx context.Context) error {
	if am.lean != nil && am.lean.OwnsValset() {
		return nil
	}
	return am.AppModule.BeginBlock(ctx)
}
