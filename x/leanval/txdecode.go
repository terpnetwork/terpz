package leanval

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// WrapTxDecoder rejects LNPR-prefixed bytes at CheckTx (vote-sdk inject-class).
func WrapTxDecoder(inner sdk.TxDecoder) sdk.TxDecoder {
	return func(txBytes []byte) (sdk.Tx, error) {
		if err := types.RejectMempoolLNPR(txBytes); err != nil {
			return nil, err
		}
		return inner(txBytes)
	}
}
