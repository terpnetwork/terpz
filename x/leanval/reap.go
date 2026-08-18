package leanval

import (
	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

func NewInsertTx(k *keeper.Keeper) func(*abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		if req != nil {
			k.NoteMembershipTx(req.Tx)
		}
		return &abci.ResponseInsertTx{}, nil
	}
}

func NewReapTxs(k *keeper.Keeper) func(*abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		txs := k.PendingMembershipTxs()
		if req != nil && req.MaxBytes > 0 {
			var out [][]byte
			var n int64
			for _, tx := range txs {
				n += int64(len(tx))
				if n > int64(req.MaxBytes) {
					break
				}
				out = append(out, tx)
			}
			txs = out
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}
