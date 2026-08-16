package ante

import (
	"bytes"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// SudoContract is the MsgSudoContract waist without importing wasmd in this crate.
type SudoContract interface {
	GetContract() string
}

// RejectLeanSudo returns ErrSudoLeanVerifier (code 2) if contract is the Lean verifier.
func RejectLeanSudo(contract []byte) error {
	if bytes.Equal(contract, types.TestLeanVerifierAcc()) {
		return types.ErrSudoLeanVerifier
	}
	return nil
}

// RejectLeanSudoRaw compares raw 20-byte addr if the caller already decoded.
func RejectLeanSudoRaw(addr []byte) error { return RejectLeanSudo(addr) }

// Decorator rejects MsgSudoContract with invalid bech32, and Lean verifier target.
type Decorator struct{}

func NewDecorator() Decorator { return Decorator{} }

func (Decorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		m, ok := msg.(*wasmtypes.MsgSudoContract)
		if !ok {
			continue
		}
		if m.Contract == "" {
			continue
		}
		addr, err := sdk.AccAddressFromBech32(m.Contract)
		if err != nil {
			return ctx, types.ErrSudoNotPermitted
		}
		if err := RejectLeanSudo(addr); err != nil {
			return ctx, err
		}
	}
	return next(ctx, tx, simulate)
}
