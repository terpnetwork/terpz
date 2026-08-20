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

// Hidden withdrawals (LEAN-7): consensus stores H(addr, secret), not the
// withdrawal address. Daily no-withdraw accumulator is a named STWO proof
// (magic STWO, prover_id=2, curve_id=5, kind NWDA). Partial withdraw is a
// separate PWDW proof. Dummy DSTW always fails. Prove off-chain; verify via
// zk-wasmvm host. BondedSet/bitfield stays power SoT; JOIN/LEAV remain LNPR.

func WithdrawCommit(addr, secret []byte) []byte {
	h := sha256.New()
	_, _ = h.Write([]byte("leanval/withdraw/commit/v1"))
	_, _ = h.Write(addr)
	_, _ = h.Write(secret)
	return h.Sum(nil)
}

func NoWithdrawAcc(period uint64, prev, commit []byte) []byte {
	var per [8]byte
	binary.BigEndian.PutUint64(per[:], period)
	if len(prev) < 32 {
		z := make([]byte, 32)
		copy(z, prev)
		prev = z
	}
	h := sha256.New()
	_, _ = h.Write([]byte("leanval/withdraw/nwacc/v1"))
	_, _ = h.Write(per[:])
	_, _ = h.Write(prev[:32])
	_, _ = h.Write(commit)
	return h.Sum(nil)
}

func PartialNewCommit(period uint64, old []byte, amount uint64) []byte {
	var per [8]byte
	binary.BigEndian.PutUint64(per[:], period)
	var amt [8]byte
	binary.BigEndian.PutUint64(amt[:], amount)
	h := sha256.New()
	_, _ = h.Write([]byte("leanval/withdraw/partial/v1"))
	_, _ = h.Write(per[:])
	_, _ = h.Write(old)
	_, _ = h.Write(amt[:])
	return h.Sum(nil)
}

func (k *Keeper) PutWithdrawCommit(period uint64, commit []byte) {
	if len(commit) == 0 {
		return
	}
	k.live().Set(types.WithdrawCommitKey(period), append([]byte(nil), commit...))
}

func (k *Keeper) GetWithdrawCommit(period uint64) []byte {
	return k.live().Get(types.WithdrawCommitKey(period))
}

func (k *Keeper) PutNoWithdrawAcc(period uint64, acc []byte) {
	if len(acc) == 0 {
		return
	}
	k.live().Set(types.NoWithdrawAccKey(period), append([]byte(nil), acc...))
}

func (k *Keeper) GetNoWithdrawAcc(period uint64) []byte {
	return k.live().Get(types.NoWithdrawAccKey(period))
}

func withdrawBin() string {
	if p := os.Getenv("LEAN_WITHDRAW"); p != "" {
		return p
	}
	if p, err := exec.LookPath("lean-withdraw"); err == nil {
		return p
	}
	for _, cand := range []string{
		"crates/lean-stwo-dummy/target/release/lean-withdraw",
		"../crates/lean-stwo-dummy/target/release/lean-withdraw",
		"../../crates/lean-stwo-dummy/target/release/lean-withdraw",
		"../../../crates/lean-stwo-dummy/target/release/lean-withdraw",
	} {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}

func pad32(b []byte) []byte {
	if len(b) >= 32 {
		return b[:32]
	}
	z := make([]byte, 32)
	copy(z, b)
	return z
}

func checkNamedStwo(proof []byte, kind []byte, what string) error {
	if bytes.Contains(proof, []byte("DSTW")) {
		return fmt.Errorf("leanval: Dummy-N is not a %s proof", what)
	}
	if len(proof) < 10 || !bytes.Equal(proof[:4], []byte("STWO")) {
		return fmt.Errorf("leanval: %s proof must be STWO magic", what)
	}
	if proof[4] != 2 || proof[5] != 5 {
		return fmt.Errorf("leanval: %s wants prover_id=2 curve_id=5", what)
	}
	if !bytes.Equal(proof[6:10], kind) {
		return fmt.Errorf("leanval: %s kind missing", what)
	}
	return nil
}

func ProveNoWithdraw(period uint64, prev, commit []byte) (acc, proof []byte, err error) {
	bin := withdrawBin()
	if bin == "" {
		return nil, nil, fmt.Errorf("leanval: lean-withdraw not on PATH")
	}
	prev = pad32(prev)
	cmd := exec.Command(
		bin, "prove-nw",
		strconv.FormatUint(period, 10),
		fmt.Sprintf("%x", prev),
		fmt.Sprintf("%x", commit),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("leanval: no-withdraw prove: %w", err)
	}
	if len(out) < 32+10 || !bytes.Equal(out[32:36], []byte("STWO")) {
		return nil, nil, fmt.Errorf("leanval: no-withdraw prove missing STWO")
	}
	if bytes.Contains(out, []byte("DSTW")) {
		return nil, nil, fmt.Errorf("leanval: no-withdraw prove emitted Dummy")
	}
	return out[:32], out[32:], nil
}

func VerifyNoWithdrawProof(proof []byte, period uint64, acc, prev []byte) error {
	if err := checkNamedStwo(proof, []byte("NWDA"), "no-withdraw"); err != nil {
		return err
	}
	prev = pad32(prev)
	acc = pad32(acc)
	inst := []byte{}
	if len(proof) >= 82 {
		inst = proof[10:82]
	}
	if err := verifyStwoInProcess(proof, inst); err == nil {
		return nil
	}
	bin := withdrawBin()
	if bin == "" {
		return fmt.Errorf("leanval: lean-withdraw not on PATH (not cargo)")
	}
	cmd := exec.Command(
		bin, "verify-nw",
		strconv.FormatUint(period, 10),
		fmt.Sprintf("%x", acc),
		fmt.Sprintf("%x", prev),
	)
	cmd.Stdin = bytes.NewReader(proof)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("leanval: no-withdraw verify: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func ProvePartialWithdraw(period uint64, old []byte, amount uint64) (next, proof []byte, err error) {
	bin := withdrawBin()
	if bin == "" {
		return nil, nil, fmt.Errorf("leanval: lean-withdraw not on PATH")
	}
	cmd := exec.Command(
		bin, "prove-pw",
		strconv.FormatUint(period, 10),
		fmt.Sprintf("%x", old),
		strconv.FormatUint(amount, 10),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("leanval: partial prove: %w", err)
	}
	if len(out) < 32+10 || !bytes.Equal(out[32:36], []byte("STWO")) {
		return nil, nil, fmt.Errorf("leanval: partial prove missing STWO")
	}
	if bytes.Contains(out, []byte("DSTW")) {
		return nil, nil, fmt.Errorf("leanval: partial prove emitted Dummy")
	}
	if bytes.Equal(out[32+6:32+10], []byte("NWDA")) {
		return nil, nil, fmt.Errorf("leanval: partial must not be NWDA")
	}
	return out[:32], out[32:], nil
}

func VerifyPartialWithdrawProof(proof []byte, period uint64, old, next []byte) error {
	if err := checkNamedStwo(proof, []byte("PWDW"), "partial-withdraw"); err != nil {
		return err
	}
	inst := []byte{}
	if len(proof) >= 82 {
		inst = proof[10:82]
	}
	if err := verifyStwoInProcess(proof, inst); err == nil {
		return nil
	}
	bin := withdrawBin()
	if bin == "" {
		return fmt.Errorf("leanval: lean-withdraw not on PATH (not cargo)")
	}
	cmd := exec.Command(
		bin, "verify-pw",
		strconv.FormatUint(period, 10),
		fmt.Sprintf("%x", old),
		fmt.Sprintf("%x", next),
	)
	cmd.Stdin = bytes.NewReader(proof)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("leanval: partial verify: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func (k *Keeper) RegisterNoWithdraw(period uint64, prev, commit []byte) ([]byte, error) {
	acc, proof, err := ProveNoWithdraw(period, prev, commit)
	if err != nil {
		return nil, err
	}
	if err := VerifyNoWithdrawProof(proof, period, acc, prev); err != nil {
		return nil, err
	}
	k.PutWithdrawCommit(period, commit)
	k.PutNoWithdrawAcc(period, acc)
	return acc, nil
}
