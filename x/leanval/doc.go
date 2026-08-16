// Package leanval is Lean v1 consensus power (worktree feat/lean-consensus only).
//
// Power for period P is BondedSet(P) only. Missing proof ⇒ weight 0.
// ValidatorUpdates come from that set, never LastValidatorPowers.
//
// ABCI compose with x/hashmerchant:
//
//	txs[0] == HMVE (0x484D5645) → LNPR (0x4C4E5052) is txs[1]
//	else LNPR is txs[0]
//
// Use keeper.WrapPrepareProposal / WrapProcessProposal around hashmerchant handlers.
// Period = height / 600 (~1h at 6s).
//
// leanval_owns_valset (default off): when on, WrapStakingEndBlock skips staking
// EndBlock; Keeper.EndBlock emits pending ValidatorUpdates.
// MsgSudoContract to TestLeanVerifierAcc is rejected (code 2) via ante.RejectLeanSudo.
package leanval
