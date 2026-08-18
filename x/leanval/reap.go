package leanval

import (
	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

func NewInsertTx(_ *keeper.Keeper) func(*abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
	return func(*abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		return &abci.ResponseInsertTx{}, nil
	}
}

func NewReapTxs(_ *keeper.Keeper) func(*abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
	return func(*abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		return &abci.ResponseReapTxs{}, nil
	}
}
