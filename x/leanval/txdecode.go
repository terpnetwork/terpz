package leanval

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

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

// CheckTx accepts JOIN/LEAV without RunTx (GetMsgs is empty; apply is PreBlock).
// Other txs use the default runTx path.
func CheckTx(runTx sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	if types.IsMembershipTx(req.Tx) {
		if j, ok := types.DecodeJoin(req.Tx); ok {
			if err := types.ValidateJoin(j); err != nil {
				return &abci.ResponseCheckTx{Code: 1, Log: err.Error()}, nil
			}
		} else if l, ok := types.DecodeLeave(req.Tx); ok {
			if err := types.ValidateLeave(l); err != nil {
				return &abci.ResponseCheckTx{Code: 1, Log: err.Error()}, nil
			}
		} else {
			return &abci.ResponseCheckTx{Code: 1, Log: "leanval: bad membership tx"}, nil
		}
		return &abci.ResponseCheckTx{Code: 0, GasWanted: 0}, nil
	}
	gInfo, result, anteEvents, err := runTx(req.Tx, nil)
	if err != nil {
		return sdkerrors.ResponseCheckTxWithEvents(err, gInfo.GasWanted, gInfo.GasUsed, anteEvents, false), nil
	}
	if result == nil {
		return &abci.ResponseCheckTx{Code: 0, GasWanted: int64(gInfo.GasWanted), GasUsed: int64(gInfo.GasUsed)}, nil
	}
	return &abci.ResponseCheckTx{
		Code:      0,
		GasWanted: int64(gInfo.GasWanted),
		GasUsed:   int64(gInfo.GasUsed),
		Log:       result.Log,
		Data:      result.Data,
	}, nil
}
