package keeper

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Unverifiable LNPR Process REJECT is the expected stall. The next round's
// valid LNPR (JOIN extras with Dummy) must ACCEPT and Apply must set the bit.
func TestProcessRejectThenValidJoinResumes(t *testing.T) {
	_ = stallJoinDir(t)
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.AllowDummy = true
	g0, g1, j0, _ := stallGenesisTwo(t, k)
	roots := k.LastObjectRoots()
	period := types.PeriodFromHeight(4)
	bad := types.LNPRBlob{
		Period: period,
		Subjects: []types.SubjectProof{
			{Subject: g0, Weight: 10, Proof: DummyStwoProveBoundRoots(period, g0, 10, roots)},
			{Subject: g1, Weight: 10, Proof: DummyStwoProveBoundRoots(period, g1, 10, roots)},
			{Subject: j0, Weight: 10, Proof: nil},
		},
	}
	if processStatus(t, k, 4, [][]byte{types.EncodeLNPR(bad)}) != abci.ResponseProcessProposal_REJECT {
		t.Fatal("round 1 must REJECT unprovable extra")
	}
	if n := bitCount(k); n != 2 {
		t.Fatalf("stall must not commit JOIN bits=%d", n)
	}

	k.NoteMembershipTx(types.EncodeJoin(types.JoinBlob{Period: 0, Subject: j0, Weight: 10}))
	resp, err := k.WrapPrepareProposal(nil)(&abci.RequestPrepareProposal{Height: 5})
	if err != nil {
		t.Fatal(err)
	}
	if processStatus(t, k, 5, resp.Txs) != abci.ResponseProcessProposal_REJECT {
		t.Fatal("subsequent round Dummy extras must REJECT")
	}
	applyJoinTxs(t, k, resp.Txs)
	if n := bitCount(k); n != 3 {
		t.Fatalf("JOIN after stall resume bits=%d want 3", n)
	}
}
