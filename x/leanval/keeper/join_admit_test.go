package keeper

import (
	"strings"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Dummy JOIN extras are a reject fixture. Unit tests that still need a live
// joiner after Dummy LNPR fails admit via AcceptProof (not Dummy-as-STWO).
func admitDummyJoinExtras(t *testing.T, k *Keeper, err error, blob types.LNPRBlob) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	if !strings.Contains(msg, "JOIN extra") && !strings.Contains(msg, "Dummy DSTW") && !strings.Contains(msg, "named STWO") {
		t.Fatal(err)
	}
	known := k.knownSubjectSet(blob.Period)
	for _, s := range blob.Subjects {
		if s.Weight <= 0 {
			continue
		}
		if _, ok := known[string(s.Subject)]; ok {
			continue
		}
		k.AcceptProof(blob.Period, s.Subject, s.Weight)
		forgetMembershipJoin(s.Subject)
	}
}

func applyJoinTxs(t *testing.T, k *Keeper, txs [][]byte) {
	t.Helper()
	err := k.ProcessInjectedLNPR(txs)
	blob, _, _ := FindLNPR(txs)
	admitDummyJoinExtras(t, k, err, blob)
}

func applyJoinBlob(t *testing.T, k *Keeper, blob types.LNPRBlob) {
	t.Helper()
	admitDummyJoinExtras(t, k, k.ApplyLNPR(blob), blob)
}
