package leanval

import (
	"fmt"
	"os"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func NewCheckTx(k *keeper.Keeper) sdk.CheckTxHandler {
	return func(runTx sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		resp, err := CheckTx(runTx, req)
		if err != nil {
			return resp, err
		}
		if resp != nil && resp.Code == 0 && req != nil {
			k.NoteMembershipTx(req.Tx)
			if types.IsMembershipTx(req.Tx) {
				fmt.Fprintf(os.Stderr, "leanval: checktx note pending=%d\n", len(k.PendingMembershipTxs()))
			}
		}
		return resp, nil
	}
}
