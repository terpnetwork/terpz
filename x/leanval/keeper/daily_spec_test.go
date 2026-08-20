package keeper

import (
	"bytes"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestDaily_DummyDSTWIsNotAProof(t *testing.T) {
	err := VerifyDailyKeyProof([]byte("DSTWDSTW"), 1, make([]byte, 32), make([]byte, 32))
	if err == nil {
		t.Fatal("Dummy DSTW must not verify as daily key")
	}
}

func TestDaily_KeyDiffersFromIdentityAndByPeriod(t *testing.T) {
	id := bytes.Repeat([]byte{0x11}, 32)
	k1 := DailyKey(7, id)
	k2 := DailyKey(8, id)
	if bytes.Equal(k1, id) {
		t.Fatal("day key must not equal persistent JOIN/LEAV subject")
	}
	if bytes.Equal(k1, k2) {
		t.Fatal("fresh key each period")
	}
	if len(k1) != 32 {
		t.Fatalf("key len %d", len(k1))
	}
}

func TestDaily_RegistryDoesNotReplaceBondedSetPower(t *testing.T) {
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	subj := []byte("join-leav-subject-32-bytes!!!!!")
	k.AcceptProof(3, subj, 100)
	k.PutDailyKey(3, DailyKey(3, subj))
	set := k.BondedSet(3)
	if len(set) != 1 || set[0].Weight != 100 {
		t.Fatalf("BondedSet remains power SoT, got %+v", set)
	}
	if bytes.Equal(k.GetDailyKey(3), subj) {
		t.Fatal("registry key is not the LNPR subject")
	}
}

func TestDaily_JoinLeaveRemainLNPRSubjects(t *testing.T) {
	join := types.EncodeJoin(types.JoinBlob{Period: 1, Subject: []byte("alice"), Weight: 10})
	leav := types.EncodeLeave(types.LeaveBlob{Period: 1, Subject: []byte("alice")})
	if !types.HasPrefix(join, types.PrefixJOIN) || !types.HasPrefix(leav, types.PrefixLEAV) {
		t.Fatal("JOIN/LEAV prefixes")
	}
	lnpr := types.EncodeLNPR(types.LNPRBlob{
		Period: 1,
		Subjects: []types.SubjectProof{{
			Subject: []byte("alice"),
			Weight:  10,
			Proof:   DummyStwoProveBound(1, []byte("alice"), 10),
		}},
	})
	if !types.HasPrefix(lnpr, types.PrefixLNPR) {
		t.Fatal("JOIN/LEAV land as LNPR subjects")
	}
}

func TestDaily_StwoProveVerify(t *testing.T) {
	if dailyBin() == "" {
		t.Skip("lean-daily-keys not built")
	}
	id := bytes.Repeat([]byte{0xab}, 32)
	prev := make([]byte, 32)
	key, proof, err := ProveDailyKey(7, id, prev)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(key, id) {
		t.Fatal("day key must not equal identity")
	}
	if err := VerifyDailyKeyProof(proof, 7, key, prev); err != nil {
		t.Fatal(err)
	}
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.PutDailyKey(7, key)
	if !bytes.Equal(k.GetDailyKey(7), key) {
		t.Fatal("registry miss")
	}
}
