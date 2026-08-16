package keeper

import "testing"

func TestVoteInfosSkipZeroWeight(t *testing.T) {
	pub := make([]byte, 32)
	pub[0] = 1
	set := []SubjectPower{
		{Subject: pub, Weight: 10, HasProof: true},
		{Subject: append([]byte{}, pub...), Weight: 0, HasProof: false},
	}
	votes, total := VoteInfosFromBondedSet(set)
	if total != 10 || len(votes) != 1 {
		t.Fatalf("got n=%d total=%d", len(votes), total)
	}
	if len(votes[0].Validator.Address) != 20 {
		t.Fatalf("cons addr len %d", len(votes[0].Validator.Address))
	}
}
