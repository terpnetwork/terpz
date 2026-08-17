package keeper

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// VerifySameStatementFold runs ONE named Stwo verify over N instances.
// Never cargo run. Dummy DSTW is fail-closed.
func VerifySameStatementFold(proof []byte, pairs []string) error {
	if bytes.Contains(proof, []byte("DSTW")) {
		return fmt.Errorf("leanval: Dummy-N is not a Stwo aggregate")
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
	bin := foldBin()
	if bin == "" {
		return fmt.Errorf("leanval: lean-stwo-fold not on PATH (not cargo)")
	}
	args := append([]string{"verify"}, pairs...)
	cmd := exec.Command(bin, args...)
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

// runnableFold returns path only if this host can exec it (reject linux ELF on macOS).
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
	// exit 1/2 from usage is fine; exec format / permission is not
	if _, ok := err.(*exec.ExitError); ok {
		return p
	}
	return ""
}

// ProveSameStatementFold runs ONE named Stwo prove over N (a,b) pairs.
func ProveSameStatementFold(pairs []string) ([]byte, error) {
	bin := foldBin()
	if bin == "" {
		return nil, fmt.Errorf("leanval: lean-stwo-fold not on PATH (not cargo)")
	}
	args := append([]string{"prove"}, pairs...)
	cmd := exec.Command(bin, args...)
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

func foldPairStrings(period uint64, subjects []types.SubjectProof, roots []byte) []string {
	out := make([]string, 0, len(subjects))
	for _, s := range subjects {
		a, b := dummySeedsBound(period, s.Subject, s.Weight, roots)
		out = append(out, fmt.Sprintf("%d,%d", a, b))
	}
	return out
}

func foldProofFromLNPR(blob types.LNPRBlob) []byte {
	for _, s := range blob.Subjects {
		if len(s.Proof) >= 10 && bytes.Equal(s.Proof[:4], []byte("STWO")) && bytes.Equal(s.Proof[6:10], []byte("FOLD")) {
			return s.Proof
		}
	}
	return nil
}
