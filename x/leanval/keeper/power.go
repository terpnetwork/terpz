package keeper

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// SubjectPower is one row of bonded_set(P).
// Missing accepted proof ⇒ Weight == 0 (never inferred from LastValidatorPowers).
type SubjectPower struct {
	Subject  []byte
	Weight   int64
	HasProof bool
}

// BondedSet returns lean_store.bonded_set(P). Single store.
func (k *Keeper) BondedSet(period uint64) []SubjectPower {
	var out []SubjectPower
	pref := types.BondedPrefixForPeriod(period)
	k.live().IteratePrefix(pref, func(key, value []byte) bool {
		subj := key[len(pref):]
		rec := decodeRec(value)
		sp := SubjectPower{Subject: append([]byte(nil), subj...)}
		if rec.accepted {
			sp.HasProof = true
			sp.Weight = rec.weight
		} else {
			sp.Weight = 0
		}
		out = append(out, sp)
		return true
	})
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i].Subject, out[j].Subject) < 0 })
	return out
}

// BondedSetOrCarry returns BondedSet(period). If the current period prefix is
// empty and period > 0, clone the last non-empty accepted rows into this period
// so height/600 does not wipe Comet VP.
func (k *Keeper) BondedSetOrCarry(period uint64) []SubjectPower {
	set := k.BondedSet(period)
	if len(set) > 0 || period == 0 {
		return set
	}
	for p := period; p > 0; p-- {
		prev := k.BondedSet(p - 1)
		if len(prev) == 0 {
			continue
		}
		for _, s := range prev {
			if s.HasProof {
				k.AcceptProof(period, s.Subject, s.Weight)
			} else {
				k.PutSubject(period, s.Subject, 0)
			}
		}
		return k.BondedSet(period)
	}
	return set
}

type rec struct {
	accepted bool
	weight   int64
}

func encodeRec(accepted bool, weight int64) []byte {
	b := make([]byte, 1+8)
	if accepted {
		b[0] = 1
	}
	copy(b[1:], types.PutI64(weight))
	return b
}

func decodeRec(v []byte) rec {
	var r rec
	if len(v) < 9 {
		return r
	}
	r.accepted = v[0] == 1
	r.weight = types.GetI64(v[1:9])
	return r
}

// PutSubject registers a subject for P without an accepted proof (weight 0).
func (k *Keeper) PutSubject(period uint64, subject []byte, claimedWeight int64) {
	k.live().Set(types.BondedKey(period, subject), encodeRec(false, claimedWeight))
}

// AcceptProof marks a verified proof for (P, subject). Weight applies only after accept.
// Also allocates a deposit-tree index and sets the participation bit (SoT).
func (k *Keeper) AcceptProof(period uint64, subject []byte, weight int64) {
	k.live().Set(types.BondedKey(period, subject), encodeRec(true, weight))
	k.admitMember(subject, weight)
}

// ValidatorUpdates emits Comet VP from set bits + EB tree (not staking shares).
//
// First Comet admission of a subject while a live set already exists is power 1.
// A silent JOIN must not take remaining VP to ≤2/3 of the new total. Power is
// never auto-raised later (EB may still record the claimed JOIN weight).
func (k *Keeper) ValidatorUpdates(period uint64) []abci.ValidatorUpdate {
	_ = period
	usingBits := len(k.live().Get(types.BitfieldKey())) > 0 || k.nextDepositIndex() > 0
	var set []SubjectPower
	if usingBits {
		set = k.DebugSubjectsFromBits()
	} else {
		set = k.BondedSetOrCarry(period)
	}
	seen := map[string]struct{}{}
	var existingTotal int64
	for _, s := range set {
		if len(s.Subject) != 32 {
			continue
		}
		// Bit-set members stay in `seen` so we never zero them just because
		// HasProof/weight is briefly 0 during JOIN Apply.
		seen[string(s.Subject)] = struct{}{}
		prev := types.GetI64(k.live().Get(types.LastPowerKey(s.Subject)))
		if prev > 0 {
			existingTotal += prev
		}
	}
	var ups []abci.ValidatorUpdate
	for _, s := range set {
		if len(s.Subject) != 32 {
			continue
		}
		if usingBits && (!s.HasProof || s.Weight == 0) {
			continue
		}
		w := s.Weight
		prev := types.GetI64(k.live().Get(types.LastPowerKey(s.Subject)))
		if prev == 0 {
			if existingTotal > 0 && w > 1 {
				w = 1
			}
		} else if prev == w {
			continue
		} else if w > prev {
			// Do not auto-raise (would turn a 1-VP JOIN into equal power).
			continue
		}
		ups = append(ups, valUpdate(s.Subject, w))
		k.live().Set(types.LastPowerKey(s.Subject), types.PutI64(w))
	}
	// Zero out last-period vals missing from this bonded set.
	k.live().IteratePrefix([]byte{types.LastUpdatesPrefix}, func(key, value []byte) bool {
		subj := key[1:]
		if _, ok := seen[string(subj)]; ok {
			return true
		}
		if types.GetI64(value) == 0 {
			return true
		}
		if len(subj) == 32 {
			ups = append(ups, valUpdate(subj, 0))
		}
		k.live().Set(types.LastPowerKey(subj), types.PutI64(0))
		return true
	})
	if len(ups) > 0 {
		fmt.Fprintf(os.Stderr, "leanval: ValidatorUpdates n=%d existing=%d\n", len(ups), existingTotal)
	}
	return ups
}

func valUpdate(pub []byte, power int64) abci.ValidatorUpdate {
	pk := append([]byte(nil), pub...)
	return abci.ValidatorUpdate{
		PubKey: cmtcrypto.PublicKey{Sum: &cmtcrypto.PublicKey_Ed25519{Ed25519: pk}},
		Power:  power,
	}
}

// hasBondedOrPrior is read-only: true if this period or any earlier period has rows.
func (k *Keeper) hasBondedOrPrior(period uint64) bool {
	if len(k.BondedSet(period)) > 0 {
		return true
	}
	for p := period; p > 0; p-- {
		if len(k.BondedSet(p-1)) > 0 {
			return true
		}
	}
	return false
}

// rejectEmptyReplace is the Linus testnet gate: empty LNPR must not wipe a live set.
func (k *Keeper) rejectEmptyReplace(blob types.LNPRBlob) error {
	if len(blob.Subjects) > 0 {
		return nil
	}
	if k.hasBondedOrPrior(blob.Period) {
		return errProof("empty LNPR replace")
	}
	return nil
}

// CheckLNPRUniqueness is Recheck-safe: empty-replace + duplicate subjects.
// Never skip this on Recheck (vote-sdk: skip crypto, never skip uniqueness).
func (k *Keeper) CheckLNPRUniqueness(blob types.LNPRBlob) error {
	if err := k.rejectEmptyReplace(blob); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(blob.Subjects))
	for _, s := range blob.Subjects {
		key := string(s.Subject)
		if _, ok := seen[key]; ok {
			return errProof("duplicate subject")
		}
		seen[key] = struct{}{}
	}
	return nil
}

// VerifyLNPR runs uniqueness then Dummy verify with store-sourced object roots.
// No store writes (ProcessProposal). skipCrypto is Recheck mode.
func (k *Keeper) VerifyLNPR(blob types.LNPRBlob) error {
	return k.verifyLNPR(blob, false)
}

// VerifyLNPRRecheck skips Dummy/FFI; still rejects empty replace and dup subjects.
func (k *Keeper) VerifyLNPRRecheck(blob types.LNPRBlob) error {
	return k.verifyLNPR(blob, true)
}

func (k *Keeper) verifyLNPR(blob types.LNPRBlob, skipCrypto bool) error {
	if err := k.CheckLNPRUniqueness(blob); err != nil {
		return err
	}
	if skipCrypto {
		return nil
	}
	if k.gas != nil {
		k.gas.ConsumeGas(stwoDummyGas, "stwo dummy verify")
	}
	if len(blob.Subjects) == 0 {
		return nil
	}
	roots := k.LastObjectRoots()
	fold := foldProofFromLNPR(blob)
	if fold != nil {
		if len(fold) > types.MaxProofBytes {
			return errProof("proof too large")
		}
		if err := VerifySameStatementFold(fold, foldPairStrings(blob.Period, blob.Subjects, roots)); err != nil {
			return err
		}
	} else if !k.AllowDummy {
		return errProof("STWO FOLD required (bitfield root, not Dummy-N)")
	}

	raw := 0
	if fold != nil {
		raw++
	}
	known := k.knownSubjectSet(blob.Period)
	for i, s := range blob.Subjects {
		if bytes.Contains(s.Proof, []byte("DSTW")) {
			if fold != nil {
				return errProof("Dummy DSTW in an aggregate must fail the batch")
			}
			_, inRoster := known[string(s.Subject)]
			if !inRoster && s.Weight > 0 && len(known) > 0 {
				return errProof("Dummy DSTW rejected on JOIN extra")
			}
		}
		if len(s.Proof) > 0 && !isFoldProof(s.Proof) {
			raw++
			if raw > types.MaxRawStarksPerLNPR {
				return errProof("too many raw STARKs per slot (aggregate or cap)")
			}
		}
		_, inRoster := known[string(s.Subject)]
		extra := !inRoster && s.Weight > 0 && len(known) > 0
		if extra {
			// Fold of current object roots does not prove JOIN subjects.
			if len(s.Proof) == 0 || isFoldProof(s.Proof) {
				return errProof("unprovable extra subject")
			}
			if bytes.Contains(s.Proof, []byte("DSTW")) {
				return errProof("Dummy DSTW rejected on JOIN extra")
			}
			if len(s.Proof) < 4 || string(s.Proof[:4]) != "STWO" {
				return errProof("JOIN extra requires named STWO (prover_id=2)")
			}
			if len(s.Proof) > types.MaxProofBytes {
				return errProof("proof too large")
			}
			if err := k.verifySubjectProof(blob.Period, uint64(i), s, roots); err != nil {
				return err
			}
			continue
		}
		if fold != nil {
			continue
		}
		if len(s.Proof) > types.MaxProofBytes {
			return errProof("proof too large")
		}
		if err := k.verifySubjectProof(blob.Period, uint64(i), s, roots); err != nil {
			return err
		}
	}
	return nil
}

func (k *Keeper) knownSubjectSet(period uint64) map[string]struct{} {
	set := k.DebugSubjectsFromBits()
	if len(set) == 0 {
		set = k.BondedSetOrCarry(period)
	}
	out := make(map[string]struct{}, len(set))
	for _, s := range set {
		out[string(s.Subject)] = struct{}{}
	}
	return out
}

func (k *Keeper) ApplyLNPR(blob types.LNPRBlob) error {
	before := countSetBits(k)
	if err := k.VerifyLNPR(blob); err != nil {
		writeLastApply("VERIFY", err.Error(), before, before, len(blob.Subjects), k.hasCtx, k.sk != nil)
		return err
	}
	// LNPR is not a replace-set of pubkeys. Bits stay unless a LEAV subject
	// (weight 0) clears them. JOIN/LEAV admit only as LNPR subjects.
	for _, s := range blob.Subjects {
		if s.Weight <= 0 {
			if _, ok := k.depositIndexOf(s.Subject); ok {
				_ = k.ApplyLeave(types.LeaveBlob{Period: blob.Period, Subject: s.Subject})
			}
			// Recheck of spent JOIN/LEAV must not keep files that rebuild the roster.
			forgetMembershipSubject(s.Subject)
			continue
		}
		if _, ok := k.depositIndexOf(s.Subject); ok {
			k.admitMember(s.Subject, s.Weight)
		} else if len(s.Subject) > 0 {
			k.AcceptProof(blob.Period, s.Subject, s.Weight)
		}
		k.live().Delete(types.PendingJoinKey(blob.Period, s.Subject))
		k.live().Delete(types.PendingJoinKey(0, s.Subject))
		forgetMembershipJoin(s.Subject)
	}
	k.syncObjectRoots()
	after := countSetBits(k)
	writeLastApply("OK", "", before, after, len(blob.Subjects), k.hasCtx, k.sk != nil)
	return nil
}

func errProof(s string) error { return fmt.Errorf("leanval: %s", s) }
