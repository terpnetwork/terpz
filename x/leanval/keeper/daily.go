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

// Daily registry (LEAN-6): per-period re-anon key, proven off-chain (STWO/M31),
// verified through zk-wasmvm host. Dummy DSTW is not a daily-key proof.
//
// Power remains BondedSet/bitfield. JOIN/LEAV stay LNPR subjects; the day key
// is not the membership subject.
//
// Leftover: Comet proposer-key wipe each period is not in this change (would
// replace ed25519 consensus keys mid-height). Registry + prove/verify only.

func DailyKey(period uint64, identity []byte) []byte {
	var per [8]byte
	binary.BigEndian.PutUint64(per[:], period)
	h := sha256.New()
	_, _ = h.Write([]byte("leanval/daily/key/v1"))
	_, _ = h.Write(per[:])
	_, _ = h.Write(identity)
	return h.Sum(nil)
}

func (k *Keeper) PutDailyKey(period uint64, key []byte) {
	if len(key) == 0 {
		return
	}
	k.live().Set(types.DailyKeyKey(period), append([]byte(nil), key...))
}

func (k *Keeper) GetDailyKey(period uint64) []byte {
	return k.live().Get(types.DailyKeyKey(period))
}

func dailyBin() string {
	if p := os.Getenv("LEAN_DAILY_KEYS"); p != "" {
		return p
	}
	if p, err := exec.LookPath("lean-daily-keys"); err == nil {
		return p
	}
	for _, cand := range []string{
		"crates/lean-stwo-dummy/target/release/lean-daily-keys",
		"../crates/lean-stwo-dummy/target/release/lean-daily-keys",
		"../../crates/lean-stwo-dummy/target/release/lean-daily-keys",
		"../../../crates/lean-stwo-dummy/target/release/lean-daily-keys",
	} {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}

// ProveDailyKey returns (day_key, STWO proof) for identity at period.
func ProveDailyKey(period uint64, identity, prevRoot []byte) (key, proof []byte, err error) {
	bin := dailyBin()
	if bin == "" {
		return nil, nil, fmt.Errorf("leanval: lean-daily-keys not on PATH")
	}
	if len(prevRoot) < 32 {
		z := make([]byte, 32)
		copy(z, prevRoot)
		prevRoot = z
	}
	cmd := exec.Command(
		bin, "prove",
		strconv.FormatUint(period, 10),
		fmt.Sprintf("%x", identity),
		fmt.Sprintf("%x", prevRoot[:32]),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("leanval: daily prove: %w", err)
	}
	if len(out) < 32+10 || !bytes.Equal(out[32:36], []byte("STWO")) {
		return nil, nil, fmt.Errorf("leanval: daily prove missing STWO")
	}
	if bytes.Contains(out, []byte("DSTW")) {
		return nil, nil, fmt.Errorf("leanval: daily prove emitted Dummy")
	}
	return out[:32], out[32:], nil
}

// VerifyDailyKeyProof checks a named STWO daily-key proof. Dummy DSTW always fails.
func VerifyDailyKeyProof(proof []byte, period uint64, dayKey, prevRoot []byte) error {
	if bytes.Contains(proof, []byte("DSTW")) {
		return fmt.Errorf("leanval: Dummy-N is not a daily-key proof")
	}
	if len(proof) < 10 || !bytes.Equal(proof[:4], []byte("STWO")) {
		return fmt.Errorf("leanval: daily proof must be STWO magic")
	}
	if proof[4] != 2 || proof[5] != 5 {
		return fmt.Errorf("leanval: daily wants prover_id=2 curve_id=5")
	}
	if !bytes.Equal(proof[6:10], []byte("DAYK")) {
		return fmt.Errorf("leanval: daily kind missing")
	}
	if len(prevRoot) < 32 {
		z := make([]byte, 32)
		copy(z, prevRoot)
		prevRoot = z
	}
	inst := []byte{}
	if len(proof) >= 82 {
		inst = proof[10:82]
	}
	if err := verifyStwoInProcess(proof, inst); err == nil {
		return nil
	}
	bin := dailyBin()
	if bin == "" {
		return fmt.Errorf("leanval: lean-daily-keys not on PATH (not cargo)")
	}
	cmd := exec.Command(
		bin, "verify",
		strconv.FormatUint(period, 10),
		fmt.Sprintf("%x", dayKey),
		fmt.Sprintf("%x", prevRoot[:32]),
	)
	cmd.Stdin = bytes.NewReader(proof)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("leanval: daily verify: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func (k *Keeper) RegisterDailyKey(period uint64, identity, prevRoot []byte) ([]byte, error) {
	key, proof, err := ProveDailyKey(period, identity, prevRoot)
	if err != nil {
		return nil, err
	}
	if err := VerifyDailyKeyProof(proof, period, key, prevRoot); err != nil {
		return nil, err
	}
	k.PutDailyKey(period, key)
	return key, nil
}
