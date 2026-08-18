package keeper

import (
	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func (k *Keeper) ApplyJoin(b types.JoinBlob) error {
	if err := types.ValidateJoin(b); err != nil {
		return err
	}
	// Admit = AllocateIndex + BitSet only (AcceptProof). No Comet req.Txs.
	k.store.Set(types.PendingJoinKey(b.Period, b.Subject), types.PutI64(b.Weight))
	k.AcceptProof(b.Period, b.Subject, b.Weight)
	return nil
}

func (k *Keeper) ApplyLeave(b types.LeaveBlob) error {
	if err := types.ValidateLeave(b); err != nil {
		return err
	}
	k.store.Set(types.PendingLeaveKey(b.Period, b.Subject), []byte{1})
	k.store.Delete(types.BondedKey(b.Period, b.Subject))
	if idx, ok := k.depositIndexOf(b.Subject); ok {
		k.BitClear(idx)
		k.SetEB(idx, 0)
	}
	return nil
}

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

// NoteMembershipTx is a no-op: CheckTx must not be the admit path.
func (k *Keeper) NoteMembershipTx(_ []byte) {}

func (k *Keeper) PendingMembershipTxs() [][]byte { return nil }

func (k *Keeper) StoredMembershipTxs(period uint64) [][]byte {
	var out [][]byte
	for _, per := range []uint64{period, 0} {
		pref := types.PendingJoinPrefixForPeriod(per)
		k.store.IteratePrefix(pref, func(key, value []byte) bool {
			subj := key[len(pref):]
			w := types.GetI64(value)
			out = append(out, types.EncodeJoin(types.JoinBlob{Period: per, Subject: subj, Weight: w}))
			return true
		})
	}
	return out
}

func (k *Keeper) ClearPendingMembership() {}
