package keeper

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SDKWrapPrepare wraps hashmerchant Prepare (ctx, req) without replacing it.
func (k *Keeper) SDKWrapPrepare(inner sdk.PrepareProposalHandler) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		k.BindContext(ctx)
		var p PrepareHandler
		if inner != nil {
			p = func(r *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
				return inner(ctx, r)
			}
		}
		resp, err := k.WrapPrepareProposal(p)(req)
		pending := len(k.PendingMembershipTxs())
		subj := 0
		if resp != nil {
			if blob, _, ok := FindLNPR(resp.Txs); ok {
				subj = len(blob.Subjects)
			}
		}
		nreq := 0
		h := int64(0)
		if req != nil {
			nreq = len(req.Txs)
			h = req.Height
		}
		ctx.Logger().Error("leanval: prepare", "h", h, "pending", pending, "lnpr_subjects", subj, "req_txs", nreq)
		return resp, err
	}
}

// SDKWrapProcess wraps hashmerchant Process. Bind KV + charge DummyStwo gas.
func (k *Keeper) SDKWrapProcess(inner sdk.ProcessProposalHandler) sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		k.BindContext(ctx)
		k.SetGasMeter(ctx.GasMeter())
		defer k.SetGasMeter(nil)
		var p ProcessHandler
		if inner != nil {
			p = func(r *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
				return inner(ctx, r)
			}
		}
		return k.WrapProcessProposal(p)(req)
	}
}
