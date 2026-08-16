package types

import "errors"

// Codespace leanval. Testers: MsgSudoContract to Lean verifier → code 2.
const (
	Codespace            = ModuleName
	CodeSudoLeanVerifier = 2
	CodeSudoNotPermitted = 3
)

var (
	ErrSudoLeanVerifier = errors.New("MsgSudoContract targeting Lean verifier is forbidden")
	ErrSudoNotPermitted = errors.New("only x/leanval may sudo the Lean verifier")
)
