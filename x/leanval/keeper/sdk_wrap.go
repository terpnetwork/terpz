package keeper

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SDKWrapPrepare wraps hashmerchant Prepare (ctx, req) without replacing it.
func (k *Keeper) SDKWrapPrepare(inner sdk.PrepareProposalHandler) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		var p PrepareHandler
		if inner != nil {
			p = func(r *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
				return inner(ctx, r)
			}
		}
		return k.WrapPrepareProposal(p)(req)
	}
}

// SDKWrapProcess wraps hashmerchant Process.
func (k *Keeper) SDKWrapProcess(inner sdk.ProcessProposalHandler) sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		var p ProcessHandler
		if inner != nil {
			p = func(r *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
				return inner(ctx, r)
			}
		}
		return k.WrapProcessProposal(p)(req)
	}
}
