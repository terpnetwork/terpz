package leanval

import (
	"context"

	"cosmossdk.io/core/comet"

	sdk "github.com/cosmos/cosmos-sdk/types"
	slashing "github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

// SlashAppModule runs stock slashing only for cons addresses that exist in
// x/staking. JOIN admits a Comet validator that is not a staking validator;
// HandleValidatorSignature → IsValidatorJailed would otherwise return
// "validator does not exist" and panic FinalizeBlock.
type SlashAppModule struct {
	slashing.AppModule
	lean *keeper.Keeper
	sk   slashingkeeper.Keeper
	cons ConsLookup
}

func WrapSlashing(am slashing.AppModule, lean *keeper.Keeper, sk slashingkeeper.Keeper, cons ConsLookup) SlashAppModule {
	return SlashAppModule{AppModule: am, lean: lean, sk: sk, cons: cons}
}

func (am SlashAppModule) BeginBlock(ctx context.Context) error {
	if am.lean == nil || !am.lean.OwnsValset() || am.cons == nil {
		return am.AppModule.BeginBlock(ctx)
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	for _, voteInfo := range sdkCtx.VoteInfos() {
		consAddr := sdk.ConsAddress(voteInfo.Validator.Address)
		if _, err := am.cons.ValidatorByConsAddr(ctx, consAddr); err != nil {
			continue
		}
		if err := am.sk.HandleValidatorSignature(
			ctx,
			voteInfo.Validator.Address,
			voteInfo.Validator.Power,
			comet.BlockIDFlag(voteInfo.BlockIdFlag),
		); err != nil {
			return err
		}
	}
	return nil
}
