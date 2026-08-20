package keeper

import (
	"bytes"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// A JOIN that is not yet signing must enter Comet at power 1 so the live
// genesis set keeps >2/3 of the new total.
func TestJoinAdmissionDoesNotThreatenQuorum(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.SetOwnsValset(true)
	g0 := bytes.Repeat([]byte{0xa1}, 32)
	g1 := bytes.Repeat([]byte{0xa2}, 32)
	g2 := bytes.Repeat([]byte{0xa3}, 32)
	g3 := bytes.Repeat([]byte{0xa4}, 32)
	join := bytes.Repeat([]byte{0xb1}, 32)
	for _, s := range [][]byte{g0, g1, g2, g3} {
		k.AcceptProof(0, s, 10)
	}
	ups := k.ValidatorUpdates(0)
	if len(ups) != 4 {
		t.Fatalf("genesis updates=%d want 4", len(ups))
	}
	for _, u := range ups {
		if u.Power != 10 {
			t.Fatalf("genesis power=%d want 10", u.Power)
		}
	}

	k.AcceptProof(0, join, 10) // claimed weight 10 — must still admit at 1
	ups = k.ValidatorUpdates(0)
	if len(ups) != 1 {
		t.Fatalf("JOIN updates=%d want 1", len(ups))
	}
	if ups[0].Power != 1 {
		t.Fatalf("JOIN Comet power=%d want 1 (silent JOIN must not equal genesis)", ups[0].Power)
	}

	// Subsequent EndBlock must not raise the JOIN to EB=10.
	ups = k.ValidatorUpdates(0)
	if len(ups) != 0 {
		t.Fatalf("JOIN power must not auto-raise: got %d updates power=%d", len(ups), ups[0].Power)
	}

	// Remaining last-power 40 > 2/3 of 41.
	var live int64
	for _, s := range k.DebugSubjectsFromBits() {
		live += types.GetI64(k.live().Get(types.LastPowerKey(s.Subject)))
	}
	if live != 41 {
		t.Fatalf("live last-power=%d want 41 (4×10 + 1)", live)
	}
}
