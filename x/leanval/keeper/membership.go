package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func membershipDir() string {
	if os.Getenv("LEANVAL_MEMBERSHIP_DISABLE") == "1" {
		return ""
	}
	if d := os.Getenv("LEANVAL_MEMBERSHIP_DIR"); d != "" {
		return d
	}
	if st, err := os.Stat("/terpd"); err == nil && st.IsDir() {
		return "/terpd/lean-join"
	}
	return filepath.Join(os.TempDir(), "terp-leanval-membership")
}

func persistEnabled() bool {
	return os.Getenv("LEANVAL_MEMBERSHIP_DISABLE") != "1" && membershipDir() != ""
}

func persistMembershipFile(tx []byte) {
	if !persistEnabled() {
		return
	}
	if err := os.MkdirAll(membershipDir(), 0o755); err != nil {
		return
	}
	sum := sha256.Sum256(tx)
	_ = os.WriteFile(filepath.Join(membershipDir(), hex.EncodeToString(sum[:])+".tx"), tx, 0o644)
}

func loadMembershipFiles() [][]byte {
	if !persistEnabled() {
		return nil
	}
	ents, err := os.ReadDir(membershipDir())
	if err != nil {
		return nil
	}
	var out [][]byte
	for _, e := range ents {
		if e.IsDir() || e.Name() == "last-prepare" || e.Name() == "last-process" || e.Name() == "last-apply" {
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
	dir := membershipDir()
	if dir == "" {
		return
	}
	_ = os.RemoveAll(dir)
}

var membershipQ struct {
	mu  sync.Mutex
	txs [][]byte
}

func (k *Keeper) ApplyJoin(b types.JoinBlob) error {
	if err := types.ValidateJoin(b); err != nil {
		return err
	}
	k.live().Set(types.PendingJoinKey(b.Period, b.Subject), types.PutI64(b.Weight))
	k.AcceptProof(b.Period, b.Subject, b.Weight)
	return nil
}

func (k *Keeper) ApplyLeave(b types.LeaveBlob) error {
	if err := types.ValidateLeave(b); err != nil {
		return err
	}
	k.live().Set(types.PendingLeaveKey(b.Period, b.Subject), []byte{1})
	k.live().Delete(types.BondedKey(b.Period, b.Subject))
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

// NoteMembershipTx records JOIN/LEAV for the next LNPR. Not Comet admission.
func NoteMembershipBytes(tx []byte) {
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

func (k *Keeper) NoteMembershipTx(tx []byte) {
	NoteMembershipBytes(tx)
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
		k.live().IteratePrefix(pref, func(key, value []byte) bool {
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

type lastPrepareFile struct {
	Pending      int `json:"pending"`
	LNPRSubjects int `json:"lnpr_subjects"`
}

func writeLastPrepare(pending, lnprSubjects int) {
	dir := membershipDir()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	bz, err := json.Marshal(lastPrepareFile{Pending: pending, LNPRSubjects: lnprSubjects})
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "last-prepare"), append(bz, '\n'), 0o644)
}

func (k *Keeper) applyQueuedMembership(period uint64, set []SubjectPower) []SubjectPower {
	leave := map[string][]byte{}
	var joins []SubjectPower
	k.live().IteratePrefix(types.PendingLeavePrefixForPeriod(period), func(key, _ []byte) bool {
		pref := types.PendingLeavePrefixForPeriod(period)
		subj := append([]byte(nil), key[len(pref):]...)
		leave[string(subj)] = subj
		return true
	})
	for _, tx := range k.PendingMembershipTxs() {
		if l, ok := types.DecodeLeave(tx); ok {
			if err := types.ValidateLeave(l); err != nil {
				continue
			}
			leave[string(l.Subject)] = append([]byte(nil), l.Subject...)
			continue
		}
		if j, ok := types.DecodeJoin(tx); ok {
			if err := types.ValidateJoin(j); err != nil {
				continue
			}
			// Pending JOIN files are LNPR subjects now. Skipping a non-zero
			// period would propose genesis-only while files exist.
			joins = append(joins, SubjectPower{Subject: j.Subject, Weight: j.Weight, HasProof: true})
		}
	}
	inSet := make(map[string]struct{}, len(set))
	for _, s := range set {
		inSet[string(s.Subject)] = struct{}{}
	}
	out := make([]SubjectPower, 0, len(set)+len(joins)+len(leave))
	seen := map[string]struct{}{}
	for _, s := range set {
		if _, drop := leave[string(s.Subject)]; drop {
			s.Weight = 0
			s.HasProof = true
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
	// LEAV admits as a weight-0 LNPR subject only while the bit is still set.
	for key, subj := range leave {
		if _, ok := seen[key]; ok {
			continue
		}
		if _, was := inSet[key]; !was {
			continue
		}
		out = append(out, SubjectPower{Subject: subj, Weight: 0, HasProof: true})
		seen[key] = struct{}{}
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i].Subject, out[j].Subject) < 0 })
	return out
}
