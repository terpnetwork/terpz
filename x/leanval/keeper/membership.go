package keeper

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func membershipDir() string {
	return filepath.Join(os.TempDir(), "terp-leanval-membership")
}

func persistMembershipFile(tx []byte) {
	if err := os.MkdirAll(membershipDir(), 0o755); err != nil {
		return
	}
	sum := sha256.Sum256(tx)
	_ = os.WriteFile(filepath.Join(membershipDir(), hex.EncodeToString(sum[:])+".tx"), tx, 0o644)
}

func loadMembershipFiles() [][]byte {
	ents, err := os.ReadDir(membershipDir())
	if err != nil {
		return nil
	}
	var out [][]byte
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		bz, err := os.ReadFile(filepath.Join(membershipDir(), e.Name()))
		if err != nil || !types.IsMembershipTx(bz) {
			continue
		}
		out = append(out, bz)
	}
	return out
}

func clearMembershipFiles() {
	_ = os.RemoveAll(membershipDir())
}

// Process-wide + tmp-file membership queue. CheckTx and Prepare must see the
// same JOIN/LEAV even across keeper reconstruction in one process.
var membershipQ struct {
	mu  sync.Mutex
	txs [][]byte
}

func (k *Keeper) ApplyJoin(b types.JoinBlob) error {
	if err := types.ValidateJoin(b); err != nil {
		return err
	}
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

func (k *Keeper) NoteMembershipTx(tx []byte) {
	if !types.IsMembershipTx(tx) {
		return
	}
	membershipQ.mu.Lock()
	defer membershipQ.mu.Unlock()
	for _, existing := range membershipQ.txs {
		if string(existing) == string(tx) {
			persistMembershipFile(tx)
			return
		}
	}
	membershipQ.txs = append(membershipQ.txs, append([]byte(nil), tx...))
	persistMembershipFile(tx)
}

func (k *Keeper) PendingMembershipTxs() [][]byte {
	membershipQ.mu.Lock()
	ram := make([][]byte, len(membershipQ.txs))
	for i, tx := range membershipQ.txs {
		ram[i] = append([]byte(nil), tx...)
	}
	membershipQ.mu.Unlock()
	seen := map[string]struct{}{}
	var out [][]byte
	for _, tx := range ram {
		seen[string(tx)] = struct{}{}
		out = append(out, tx)
	}
	for _, tx := range loadMembershipFiles() {
		if _, ok := seen[string(tx)]; ok {
			continue
		}
		out = append(out, tx)
	}
	return out
}

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

func (k *Keeper) ClearPendingMembership() {
	membershipQ.mu.Lock()
	defer membershipQ.mu.Unlock()
	membershipQ.txs = nil
	clearMembershipFiles()
}

func (k *Keeper) applyQueuedMembership(period uint64, set []SubjectPower) []SubjectPower {
	leave := map[string]struct{}{}
	var joins []SubjectPower
	k.store.IteratePrefix(types.PendingLeavePrefixForPeriod(period), func(key, _ []byte) bool {
		pref := types.PendingLeavePrefixForPeriod(period)
		leave[string(key[len(pref):])] = struct{}{}
		return true
	})
	k.store.IteratePrefix(types.PendingJoinPrefixForPeriod(period), func(key, value []byte) bool {
		pref := types.PendingJoinPrefixForPeriod(period)
		subj := key[len(pref):]
		joins = append(joins, SubjectPower{Subject: append([]byte(nil), subj...), Weight: types.GetI64(value), HasProof: true})
		return true
	})
	if period != 0 {
		k.store.IteratePrefix(types.PendingJoinPrefixForPeriod(0), func(key, value []byte) bool {
			pref := types.PendingJoinPrefixForPeriod(0)
			subj := key[len(pref):]
			joins = append(joins, SubjectPower{Subject: append([]byte(nil), subj...), Weight: types.GetI64(value), HasProof: true})
			return true
		})
	}
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
