package leanval

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// WrapTxDecoder rejects LNPR-prefixed bytes at CheckTx (vote-sdk inject-class).
// JOIN/LEAV are committed membership txs: decode to MembershipTx (mempool-legal).
func WrapTxDecoder(inner sdk.TxDecoder) sdk.TxDecoder {
	return func(txBytes []byte) (sdk.Tx, error) {
		if err := types.RejectMempoolLNPR(txBytes); err != nil {
			return nil, err
		}
		if types.IsMembershipTx(txBytes) {
			if j, ok := types.DecodeJoin(txBytes); ok {
				if err := types.ValidateJoin(j); err != nil {
					return nil, err
				}
			} else if l, ok := types.DecodeLeave(txBytes); ok {
				if err := types.ValidateLeave(l); err != nil {
					return nil, err
				}
			} else {
				return nil, types.ErrMempoolLNPR
			}
			return types.MembershipTx{Raw: append([]byte(nil), txBytes...)}, nil
		}
		return inner(txBytes)
	}
}

// WrapAnte skips sigs/fees for JOIN/LEAV; those txs are applied in PreBlock.
func WrapAnte(inner sdk.AnteHandler) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		if _, ok := tx.(types.MembershipTx); ok {
			return ctx, nil
		}
		if inner == nil {
			return ctx, nil
		}
		return inner(ctx, tx, simulate)
	}
}
