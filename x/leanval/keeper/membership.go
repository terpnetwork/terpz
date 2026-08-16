package keeper

import "github.com/terpnetwork/terp-core/v6/x/leanval/types"

// ApplyJoin writes a BondedSet row. Next honest Prepare includes this subject
// from committed state — not from disk lean-pending.json.
func (k *Keeper) ApplyJoin(b types.JoinBlob) error {
	if err := types.ValidateJoin(b); err != nil {
		return err
	}
	k.AcceptProof(b.Period, b.Subject, b.Weight)
	return nil
}

// ApplyLeave drops the subject from this period's BondedSet.
func (k *Keeper) ApplyLeave(b types.LeaveBlob) error {
	if err := types.ValidateLeave(b); err != nil {
		return err
	}
	k.store.Delete(types.BondedKey(b.Period, b.Subject))
	return nil
}

// ProcessMembershipTxs applies JOIN/LEAV bytes already in the committed block.
// Call after ProcessInjectedLNPR so replace-set cannot wipe this block's joins.
func (k *Keeper) ProcessMembershipTxs(txs [][]byte) error {
	for _, tx := range txs {
		if j, ok := types.DecodeJoin(tx); ok {
			if err := k.ApplyJoin(j); err != nil {
				return err
			}
			continue
		}
		if l, ok := types.DecodeLeave(tx); ok {
			if err := k.ApplyLeave(l); err != nil {
				return err
			}
		}
	}
	return nil
}
