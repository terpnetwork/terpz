package types

import "errors"

// Codespace leanval. Testers: MsgSudoContract to Lean verifier → code 2.
const (
	Codespace            = ModuleName
	CodeSudoLeanVerifier = 2
	CodeSudoNotPermitted = 3
	CodeMempoolLNPR      = 4
	CodeMempoolSSLE      = 5
)

var (
	ErrSudoLeanVerifier = errors.New("MsgSudoContract targeting Lean verifier is forbidden")
	ErrSudoNotPermitted = errors.New("only x/leanval may sudo the Lean verifier")
	ErrMempoolLNPR      = errors.New("LNPR is proposer-inject only; CheckTx rejected")
	ErrMempoolSSLE      = errors.New("SSLE is proposer-inject only; CheckTx rejected")
)
