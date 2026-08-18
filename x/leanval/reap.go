package leanval

import (
	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

func NewInsertTx(k *keeper.Keeper) func(*abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		if req != nil && k != nil {
			k.NoteMembershipTx(req.Tx)
		}
		return &abci.ResponseInsertTx{}, nil
	}
}

func NewReapTxs(k *keeper.Keeper) func(*abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
	return func(*abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		var txs [][]byte
		if k != nil {
			txs = k.PendingMembershipTxs()
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}
