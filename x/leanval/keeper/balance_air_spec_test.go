package keeper

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Phase 1B daily-balance STARK I/O spec (LEAN-4 / CONSTRAINT-SYSTEM).
// Valid AIR verify is the red bar until a real Stwo balance circuit exists.

type balanceAirSpecRow struct {
	name              string
	period            uint64
	wrongPeriod       uint64
	weight            int64
	subject           []byte
	prev              [32]byte
	deposit, bits, eb [32]byte
	branch            []byte
	fine              int64
}

func phase1BSpecTable() []balanceAirSpecRow {
	subj := []byte("lean-subject-ed25519-pk-32bytes!")
	return []balanceAirSpecRow{
		{
			name:        "v1-int64-weight-zero-roots",
			period:      7,
			wrongPeriod: 8,
			weight:      21,
			subject:     subj,
			branch:      []byte("merkle-participation-stub"),
			fine:        21,
		},
	}
}

// TestPhase1B_BalanceAIR_ValidStatementAccepted is red until VerifyBalanceAir
// accepts a well-formed instance (period, prev, weight, roots, witness).
func TestPhase1B_BalanceAIR_ValidStatementAccepted(t *testing.T) {
	air := UnimplementedBalanceAir{}
	for _, row := range phase1BSpecTable() {
		t.Run(row.name, func(t *testing.T) {
			pub := BalanceAirPublic{
				Period:          row.period,
				PrevCommitment:  row.prev,
				Weight:          row.weight,
				DepositTreeRoot: row.deposit,
				BitfieldRoot:    row.bits,
				EBTreeRoot:      row.eb,
				Subject:         row.subject,
			}
			priv := BalanceAirPrivate{ParticipationBranch: row.branch, FineBalance: row.fine}
			err := air.VerifyBalanceAir(nil, pub, priv)
			if err != nil {
				t.Fatalf("Phase 1B Balance AIR not implemented (red): %v", err)
			}
		})
	}
}

// TestPhase1B_BalanceAIR_StubRejectsUnknownAIR documents the fail-closed stub.
func TestPhase1B_BalanceAIR_StubRejectsUnknownAIR(t *testing.T) {
	err := (UnimplementedBalanceAir{}).VerifyBalanceAir(nil, BalanceAirPublic{Period: 1}, BalanceAirPrivate{})
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("UnimplementedBalanceAir must return not implemented, got %v", err)
	}
}

// TestPhase1B_MissingProofWeightZero is already the BondedSet contract (green).
func TestPhase1B_MissingProofWeightZero(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	period := uint64(4)
	subj := []byte("no-proof-subject-aaaaaaaaaaaa")
	k.PutSubject(period, subj, 100)
	set := k.BondedSet(period)
	if len(set) != 1 || set[0].Weight != 0 || set[0].HasProof {
		t.Fatalf("missing proof => weight 0, got %+v", set)
	}
}

// TestPhase1B_WrongPeriodDummyMustNotAcceptProof: DummyStwo of a different
// period must not write bonded weight.
func TestPhase1B_WrongPeriodDummyMustNotAcceptProof(t *testing.T) {
	row := phase1BSpecTable()[0]
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.PutSubject(row.period, row.subject, 0)
	blob := types.LNPRBlob{
		Period: row.period,
		Subjects: []types.SubjectProof{{
			Subject: row.subject,
			Weight:  row.weight,
			Proof:   DummyStwoProveBound(row.wrongPeriod, row.subject, row.weight),
		}},
	}
	if err := k.ApplyLNPR(blob); err == nil {
		t.Fatal("wrong-period DummyStwo must fail verify")
	}
	set := k.BondedSet(row.period)
	for _, s := range set {
		if bytes.Equal(s.Subject, row.subject) && (s.HasProof || s.Weight != 0) {
			t.Fatalf("must not AcceptProof on wrong statement: %+v", s)
		}
	}
}

// TestPhase1B_InstanceLayoutPeriodWeightSubjectBE documents the public instance
// bytes DummyStwo (and a future Stwo path) must consume.
func TestPhase1B_InstanceLayoutPeriodWeightSubjectBE(t *testing.T) {
	period := uint64(0x0102030405060708)
	weight := int64(0x1112131415161718)
	subj := []byte{0xaa, 0xbb, 0xcc}
	got := BalanceAirInstanceBytes(period, weight, subj)
	want := make([]byte, 16+len(subj))
	binary.BigEndian.PutUint64(want[0:8], period)
	binary.BigEndian.PutUint64(want[8:16], uint64(weight))
	copy(want[16:], subj)
	if !bytes.Equal(got, want) {
		t.Fatalf("layout period(8 BE)|weight(8 BE)|subject\n got %x\nwant %x", got, want)
	}
	proof := DummyStwoProveBound(period, subj, weight)
	if err := (DummyStwoGo{}).VerifyDummy(proof, got); err != nil {
		t.Fatalf("DummyStwo must consume documented layout: %v", err)
	}
}

// TestPhase1B_NoCircuitTypeStark greps the worktree (rust+go, skip vendor/build).
func TestPhase1B_NoCircuitTypeStark(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	needle := "CircuitType" + "::" + "Stark"
	cmd := exec.Command("grep", "-RInF", needle,
		"--include=*.go", "--include=*.rs", root)
	out, _ := cmd.CombinedOutput()
	var hits []string
	for _, ln := range strings.Split(string(out), "\n") {
		if ln == "" {
			continue
		}
		if strings.Contains(ln, "/vendor/") || strings.Contains(ln, "/build/") || strings.Contains(ln, "/target/") {
			continue
		}
		if strings.Contains(ln, "balance_air_spec_test.go") {
			continue
		}
		if !strings.Contains(ln, needle) {
			continue
		}
		hits = append(hits, ln)
	}
	if len(hits) > 0 {
		t.Fatalf("CircuitType::Stark must not exist:\n%s", strings.Join(hits, "\n"))
	}
}
