package keeper

import (
	"bytes"
	"fmt"
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
	k.store.IteratePrefix(pref, func(key, value []byte) bool {
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
	k.store.Set(types.BondedKey(period, subject), encodeRec(false, claimedWeight))
}

// AcceptProof marks a verified proof for (P, subject). Weight applies only after accept.
func (k *Keeper) AcceptProof(period uint64, subject []byte, weight int64) {
	k.store.Set(types.BondedKey(period, subject), encodeRec(true, weight))
}

// ValidatorUpdates emits Comet updates from BondedSet only (no staking path).
func (k *Keeper) ValidatorUpdates(period uint64) []abci.ValidatorUpdate {
	set := k.BondedSet(period)
	seen := map[string]struct{}{}
	var ups []abci.ValidatorUpdate
	for _, s := range set {
		seen[string(s.Subject)] = struct{}{}
		prev := types.GetI64(k.store.Get(types.LastPowerKey(s.Subject)))
		if prev == s.Weight && prev != 0 {
			continue
		}
		ups = append(ups, valUpdate(s.Subject, s.Weight))
		k.store.Set(types.LastPowerKey(s.Subject), types.PutI64(s.Weight))
	}
	// Zero out last-period vals missing from this bonded set.
	k.store.IteratePrefix([]byte{types.LastUpdatesPrefix}, func(key, value []byte) bool {
		subj := key[1:]
		if _, ok := seen[string(subj)]; ok {
			return true
		}
		if types.GetI64(value) == 0 {
			return true
		}
		ups = append(ups, valUpdate(subj, 0))
		k.store.Set(types.LastPowerKey(subj), types.PutI64(0))
		return true
	})
	return ups
}

func valUpdate(pub []byte, power int64) abci.ValidatorUpdate {
	pk := append([]byte(nil), pub...)
	return abci.ValidatorUpdate{
		PubKey: cmtcrypto.PublicKey{Sum: &cmtcrypto.PublicKey_Ed25519{Ed25519: pk}},
		Power:  power,
	}
}

// VerifyLNPR runs VerifyDummy on each subject. No store writes (ProcessProposal).
func (k *Keeper) VerifyLNPR(blob types.LNPRBlob) error {
	for _, s := range blob.Subjects {
		if k.gas != nil {
			k.gas.ConsumeGas(stwoDummyGas, "stwo dummy verify")
		}
		if len(s.Proof) > types.MaxProofBytes {
			return errProof("proof too large")
		}
		if err := k.Verifier.VerifyDummy(s.Proof, instancesFor(blob.Period, s.Subject, s.Weight)); err != nil {
			return err
		}
	}
	return nil
}

// ApplyLNPR verifies then writes bonded_set. Fail-closed.
func (k *Keeper) ApplyLNPR(blob types.LNPRBlob) error {
	if err := k.VerifyLNPR(blob); err != nil {
		return err
	}
	for _, s := range blob.Subjects {
		k.AcceptProof(blob.Period, s.Subject, s.Weight)
	}
	return nil
}

func errProof(s string) error { return fmt.Errorf("leanval: %s", s) }
