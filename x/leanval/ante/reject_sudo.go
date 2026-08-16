package ante

import (
	"bytes"

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

// RejectLeanSudoBech32 compares raw 20-byte addr if the caller already decoded.
func RejectLeanSudoRaw(addr []byte) error { return RejectLeanSudo(addr) }
