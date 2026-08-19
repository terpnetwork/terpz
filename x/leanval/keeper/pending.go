package keeper

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// PendingFile helpers exist only for unit tests of the old ICT sidecar format.
// Production Prepare must not call mergePending / loadPending.
// Join and leave are committed by the app (ApplyLNPR / msgs), not this file.
const DefaultPendingPath = "/terpd/.terpd/config/lean-pending.json"

type pendingFile struct {
	Join  []pendingJoin `json:"join"`
	Leave []string      `json:"leave"`
}

type pendingJoin struct {
	PubKey string `json:"pubkey"`
	Weight int64  `json:"weight"`
}

func pendingPath() string {
	if p := os.Getenv("LEANVAL_PENDING"); p != "" {
		return p
	}
	cands := []string{DefaultPendingPath}
	if ents, err := os.ReadDir("/var/cosmos-chain"); err == nil {
		for _, e := range ents {
			cands = append(cands, "/var/cosmos-chain/"+e.Name()+"/config/lean-pending.json")
		}
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && st.Size() > 0 {
			return c
		}
	}
	return ""
}

func loadPending() pendingFile {
	path := pendingPath()
	if path == "" {
		return pendingFile{}
	}
	bz, err := os.ReadFile(path)
	if err != nil || len(bz) == 0 {
		return pendingFile{}
	}
	var p pendingFile
	_ = json.Unmarshal(bz, &p)
	return p
}

func decodeSubject(s string) []byte {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) > 0 {
		return b
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) > 0 {
		return b
	}
	return []byte(s)
}

func (k *Keeper) mergePending(period uint64, set []SubjectPower) []types.SubjectProof {
	roots := k.LastObjectRoots()
	pend := loadPending()
	leave := map[string]struct{}{}
	for _, s := range pend.Leave {
		if b := decodeSubject(s); len(b) > 0 {
			leave[string(b)] = struct{}{}
		}
	}
	out := make([]types.SubjectProof, 0, len(set)+len(pend.Join))
	seen := map[string]struct{}{}
	for _, s := range set {
		if _, drop := leave[string(s.Subject)]; drop {
			continue
		}
		out = append(out, types.SubjectProof{
			Subject: s.Subject,
			Weight:  s.Weight,
			Proof:   DummyStwoProveBoundRoots(period, s.Subject, s.Weight, roots),
		})
		seen[string(s.Subject)] = struct{}{}
	}
	for _, j := range pend.Join {
		subj := decodeSubject(j.PubKey)
		if len(subj) == 0 {
			continue
		}
		if _, drop := leave[string(subj)]; drop {
			continue
		}
		if _, ok := seen[string(subj)]; ok {
			continue
		}
		w := j.Weight
		if w <= 0 {
			w = 10
		}
		out = append(out, types.SubjectProof{
			Subject: subj,
			Weight:  w,
			Proof:   DummyStwoProveBoundRoots(period, subj, w, roots),
		})
		seen[string(subj)] = struct{}{}
	}
	return out
}

func (k *Keeper) dropUnlisted(period uint64, listed map[string]struct{}) {
	pref := types.BondedPrefixForPeriod(period)
	var del [][]byte
	k.live().IteratePrefix(pref, func(key, _ []byte) bool {
		subj := key[len(pref):]
		if _, ok := listed[string(subj)]; !ok {
			if k.live().Get(types.PendingJoinKey(period, subj)) != nil || k.live().Get(types.PendingJoinKey(0, subj)) != nil {
				return true
			}
			del = append(del, append([]byte(nil), key...))
		}
		return true
	})
	for _, key := range del {
		k.live().Delete(key)
	}
}
