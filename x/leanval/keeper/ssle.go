package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// HideUntilBlockSSLE is LEAN-8 SSLE: the scheduled view is a ticket, not a
// consensus address. The proposer id is available only after the block that
// carries a Stwo SSLE proof is supplied. Dummy DSTW is not a ticket proof.
type HideUntilBlockSSLE struct{}

func (HideUntilBlockSSLE) HiddenProposerID(period uint64, height int64) ([]byte, error) {
	if height < 0 {
		return nil, fmt.Errorf("leanval: ssle height")
	}
	return ssleTicket(period, height, nil), nil
}

func (HideUntilBlockSSLE) RevealProposerAfterBlock(period uint64, height int64, block []byte) ([]byte, error) {
	if height < 0 {
		return nil, fmt.Errorf("leanval: ssle height")
	}
	if len(block) == 0 {
		return nil, fmt.Errorf("leanval: ssle missing block")
	}
	blob, ok := types.DecodeSSLE(block)
	if ok && len(blob.Proposer) > 0 {
		want := ssleTicket(period, height, blob.Proposer)
		if !bytes.Equal(blob.Ticket, want) {
			return nil, fmt.Errorf("leanval: ssle ticket != H(period,height,proposer)")
		}
		if err := VerifySSLEProof(blob.Proof, period, height, blob.Ticket); err != nil {
			return nil, err
		}
		if bytes.Equal(blob.Ticket, blob.Proposer) {
			return nil, fmt.Errorf("leanval: ssle ticket must not equal proposer id")
		}
		return append([]byte(nil), blob.Proposer...), nil
	}
	return ssleReveal(period, height, block), nil
}

func ssleTicket(period uint64, height int64, proposer []byte) []byte {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], period)
	binary.BigEndian.PutUint64(buf[8:16], uint64(height))
	h := sha256.New()
	_, _ = h.Write([]byte("leanval/ssle/ticket/v1"))
	_, _ = h.Write(buf[:])
	_, _ = h.Write(proposer)
	return h.Sum(nil)
}

func ssleReveal(period uint64, height int64, block []byte) []byte {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], period)
	binary.BigEndian.PutUint64(buf[8:16], uint64(height))
	h := sha256.New()
	_, _ = h.Write([]byte("leanval/ssle/reveal/v1"))
	_, _ = h.Write(buf[:])
	_, _ = h.Write(block)
	return h.Sum(nil)
}

func ssleBin() string {
	if p := os.Getenv("LEAN_SSLE"); p != "" {
		return p
	}
	if p, err := exec.LookPath("lean-ssle"); err == nil {
		return p
	}
	for _, cand := range []string{
		"crates/lean-stwo-dummy/target/release/lean-ssle",
		"../crates/lean-stwo-dummy/target/release/lean-ssle",
		"../../crates/lean-stwo-dummy/target/release/lean-ssle",
		"../../../crates/lean-stwo-dummy/target/release/lean-ssle",
	} {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}

// ProveSSLE returns (ticket, STWO SSLE proof) for the proposer's identity.
func ProveSSLE(period uint64, height int64, proposer []byte) (ticket, proof []byte, err error) {
	bin := ssleBin()
	if bin == "" {
		return nil, nil, fmt.Errorf("leanval: lean-ssle not on PATH")
	}
	cmd := exec.Command(bin, "prove", strconv.FormatUint(period, 10), strconv.FormatInt(height, 10), fmt.Sprintf("%x", proposer))
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("leanval: ssle prove: %w", err)
	}
	if len(out) < 32+10 || !bytes.Equal(out[32:36], []byte("STWO")) {
		return nil, nil, fmt.Errorf("leanval: ssle prove missing STWO")
	}
	if bytes.Contains(out, []byte("DSTW")) {
		return nil, nil, fmt.Errorf("leanval: ssle prove emitted Dummy")
	}
	return out[:32], out[32:], nil
}

// VerifySSLEProof checks a Stwo SSLE proof against (period, height, ticket).
// Dummy DSTW always fails.
func VerifySSLEProof(proof []byte, period uint64, height int64, ticket []byte) error {
	if bytes.Contains(proof, []byte("DSTW")) {
		return fmt.Errorf("leanval: Dummy-N is not an SSLE proof")
	}
	if len(proof) < 10 || !bytes.Equal(proof[:4], []byte("STWO")) {
		return fmt.Errorf("leanval: ssle proof must be STWO magic")
	}
	if proof[4] != 2 || proof[5] != 5 {
		return fmt.Errorf("leanval: ssle wants prover_id=2 curve_id=5")
	}
	if !bytes.Equal(proof[6:10], []byte("SSLE")) {
		return fmt.Errorf("leanval: ssle kind missing")
	}
	inst := []byte{}
	if len(proof) >= 58 {
		inst = proof[10:58]
	}
	if err := verifyStwoInProcess(proof, inst); err == nil {
		return nil
	}
	bin := ssleBin()
	if bin == "" {
		return fmt.Errorf("leanval: lean-ssle not on PATH (not cargo)")
	}
	cmd := exec.Command(bin, "verify", strconv.FormatUint(period, 10), strconv.FormatInt(height, 10), fmt.Sprintf("%x", ticket))
	cmd.Stdin = bytes.NewReader(proof)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("leanval: ssle verify: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func (k *Keeper) buildSSLE(period uint64, height int64, proposer []byte) []byte {
	ticket, proof, err := ProveSSLE(period, height, proposer)
	if err != nil {
		return nil
	}
	return types.EncodeSSLE(types.SSLEBlob{
		Period:   period,
		Height:   height,
		Ticket:   ticket,
		Proposer: proposer,
		Proof:    proof,
	})
}
