package keeper

import (
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func (k *Keeper) SetStoreKey(key storetypes.StoreKey) { k.sk = key }

func (k *Keeper) BindContext(ctx sdk.Context) {
	k.sdkCtx = ctx
	k.hasCtx = true
	if k.sk != nil {
		k.store = BindKV(ctx, k.sk)
	}
	if v := k.live().Get(types.PeriodParamKey()); len(v) >= 8 {
		types.SetBlocksPerPeriod(types.GetI64(v))
	}
}
