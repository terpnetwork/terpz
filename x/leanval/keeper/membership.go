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

func (k *Keeper) NoteMembershipTx(tx []byte) {
	if !types.IsMembershipTx(tx) {
		return
	}
	for _, existing := range k.memMembership {
		if string(existing) == string(tx) {
			return
		}
	}
	k.memMembership = append(k.memMembership, append([]byte(nil), tx...))
}

func (k *Keeper) PendingMembershipTxs() [][]byte {
	out := make([][]byte, len(k.memMembership))
	copy(out, k.memMembership)
	return out
}

func (k *Keeper) applyQueuedMembership(period uint64, set []SubjectPower) []SubjectPower {
	leave := map[string]struct{}{}
	var joins []SubjectPower
	for _, tx := range k.PendingMembershipTxs() {
		if l, ok := types.DecodeLeave(tx); ok {
			leave[string(l.Subject)] = struct{}{}
			continue
		}
		if j, ok := types.DecodeJoin(tx); ok {
			if err := types.ValidateJoin(j); err != nil {
				continue
			}
			if j.Period != 0 && j.Period != period {
				continue
			}
			joins = append(joins, SubjectPower{Subject: j.Subject, Weight: j.Weight, HasProof: true})
		}
	}
	out := make([]SubjectPower, 0, len(set)+len(joins))
	seen := map[string]struct{}{}
	for _, s := range set {
		if _, drop := leave[string(s.Subject)]; drop {
			continue
		}
		out = append(out, s)
		seen[string(s.Subject)] = struct{}{}
	}
	for _, j := range joins {
		if _, drop := leave[string(j.Subject)]; drop {
			continue
		}
		if _, ok := seen[string(j.Subject)]; ok {
			continue
		}
		out = append(out, j)
		seen[string(j.Subject)] = struct{}{}
	}
	return out
}
