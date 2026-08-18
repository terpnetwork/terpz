package keeper

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func isHexRoot(s string) bool {
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func parseRootHex(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return make([]byte, 32)
	}
	return b
}

// VerifySameStatementFold runs ONE named Stwo verify over bitfield+object roots.
// pairs are hex roots (bitfield, deposit, eb) — never Dummy (a,b) costume pairs.
func VerifySameStatementFold(proof []byte, pairs []string) error {
	if bytes.Contains(proof, []byte("DSTW")) {
		return fmt.Errorf("leanval: Dummy-N is not a Stwo aggregate")
	}
	if bytes.Contains(proof, []byte("c=3a+5b+7")) {
		return fmt.Errorf("leanval: 3a+5b+7 costume over subjects is not a fold")
	}
	if len(proof) < 10 || !bytes.Equal(proof[:4], []byte("STWO")) {
		return fmt.Errorf("leanval: fold proof must be STWO magic")
	}
	if len(proof) < 6 || proof[4] != 2 || proof[5] != 5 {
		return fmt.Errorf("leanval: fold wants prover_id=2 curve_id=5")
	}
	if !bytes.Equal(proof[6:10], []byte("FOLD")) {
		return fmt.Errorf("leanval: fold kind missing")
	}
	if len(pairs) < 1 || !isHexRoot(pairs[0]) {
		return fmt.Errorf("leanval: fold PIs must be bitfield root hex, not Dummy pairs")
	}
	if len(proof) >= 10+types.ObjectRootsSize {
		want := parseRootHex(pairs[0])
		if !bytes.Equal(proof[10:42], want) {
			return fmt.Errorf("leanval: fold public inputs != bitfield root")
		}
	}
	bin := foldBin()
	if bin == "" {
		return fmt.Errorf("leanval: lean-stwo-fold not on PATH (not cargo)")
	}
	bf := pairs[0]
	dep := "00"
	eb := "00"
	if len(pairs) > 1 {
		dep = pairs[1]
	}
	if len(pairs) > 2 {
		eb = pairs[2]
	}
	cmd := exec.Command(bin, "verify", bf, dep, eb)
	cmd.Stdin = bytes.NewReader(proof)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("leanval: fold verify: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func FoldVerifyFile(path string, pairs []string) error {
	bz, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return VerifySameStatementFold(bz, pairs)
}

func foldBin() string {
	if p := os.Getenv("LEAN_STWO_FOLD"); p != "" {
		return runnableFold(p)
	}
	if p, err := exec.LookPath("lean-stwo-fold"); err == nil {
		return runnableFold(p)
	}
	return ""
}

func runnableFold(p string) string {
	if p == "" {
		return ""
	}
	cmd := exec.Command(p)
	err := cmd.Run()
	if err == nil {
		return p
	}
	if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
		return ""
	}
	if _, ok := err.(*exec.ExitError); ok {
		return p
	}
	return ""
}

func ProveSameStatementFold(pairs []string) ([]byte, error) {
	bin := foldBin()
	if bin == "" {
		return nil, fmt.Errorf("leanval: lean-stwo-fold not on PATH (not cargo)")
	}
	if len(pairs) < 1 || !isHexRoot(pairs[0]) {
		return nil, fmt.Errorf("leanval: fold prove wants bitfield root hex")
	}
	dep := strings.Repeat("00", 32)
	eb := strings.Repeat("00", 32)
	if len(pairs) > 1 && isHexRoot(pairs[1]) {
		dep = pairs[1]
	}
	if len(pairs) > 2 && isHexRoot(pairs[2]) {
		eb = pairs[2]
	}
	cmd := exec.Command(bin, "prove", pairs[0], dep, eb)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("leanval: fold prove: %w: %s", err, bytes.TrimSpace(ee.Stderr))
		}
		return nil, fmt.Errorf("leanval: fold prove: %w", err)
	}
	if bytes.Contains(out, []byte("DSTW")) {
		return nil, fmt.Errorf("leanval: fold prove emitted Dummy")
	}
	if len(out) < 10 || !bytes.Equal(out[:4], []byte("STWO")) {
		return nil, fmt.Errorf("leanval: fold prove missing STWO")
	}
	return out, nil
}

// foldPairStrings is the fold PI list: bitfield root || deposit root || EB root.
// Not N Dummy (a,b) pairs over subjects.
func foldPairStrings(period uint64, subjects []types.SubjectProof, roots []byte) []string {
	_ = period
	_ = subjects
	r := padRoots(roots)
	return []string{
		hex.EncodeToString(r[0:32]),
		hex.EncodeToString(r[32:64]),
		hex.EncodeToString(r[64:96]),
	}
}

func foldProofFromLNPR(blob types.LNPRBlob) []byte {
	for _, s := range blob.Subjects {
		if len(s.Proof) >= 10 && bytes.Equal(s.Proof[:4], []byte("STWO")) && bytes.Equal(s.Proof[6:10], []byte("FOLD")) {
			return s.Proof
		}
	}
	return nil
}
