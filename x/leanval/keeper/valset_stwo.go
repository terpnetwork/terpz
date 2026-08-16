package keeper

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func leanStwoCrateDir() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		cand := filepath.Join(root, "crates", "lean-stwo-dummy")
		if _, err := os.Stat(filepath.Join(cand, "Cargo.toml")); err == nil {
			return cand, nil
		}
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			cand = filepath.Join(root, "crates", "lean-stwo-dummy")
			if _, err := os.Stat(filepath.Join(cand, "Cargo.toml")); err == nil {
				return cand, nil
			}
		}
		root = filepath.Dir(root)
	}
	return "", fmt.Errorf("lean-stwo-dummy crate not found")
}

func indexToHex5(index uint64) string {
	return fmt.Sprintf("%010x", index)
}

func leanValsetAirBin() string {
	if p := os.Getenv("LEAN_VALSET_AIR"); p != "" {
		return p
	}
	if p, err := exec.LookPath("lean-valset-air"); err == nil {
		return p
	}
	return ""
}

func proveValsetStwo(period, index uint64, eb uint8, _ []byte) ([]byte, error) {

	if index >= (1 << 40) {
		return nil, fmt.Errorf("leanval: valset AIR deposit index exceeds 5 bytes")
	}
	if bin := leanValsetAirBin(); bin != "" {
		cmd := exec.Command(bin, "prove", strconv.FormatUint(period, 10), indexToHex5(index), strconv.FormatUint(uint64(eb), 10))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("leanval: valset prove: %w: %s", err, bytes.TrimSpace(out))
		}
		if len(out) < 4 || !bytes.Equal(out[:4], []byte("STWO")) {
			return nil, fmt.Errorf("leanval: valset prove did not emit STWO proof")
		}
		return out, nil
	}
	dir, err := leanStwoCrateDir()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(
		"cargo", "run", "--offline", "--quiet", "--features", "real-stwo",
		"--bin", "lean-valset-air", "--",
		"prove", strconv.FormatUint(period, 10), indexToHex5(index), strconv.FormatUint(uint64(eb), 10),
	)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("leanval: valset prove: %w: %s", err, bytes.TrimSpace(out))
	}
	if len(out) < 4 || !bytes.Equal(out[:4], []byte("STWO")) {
		return nil, fmt.Errorf("leanval: valset prove did not emit STWO proof")
	}
	return out, nil
}

func verifyValsetStwo(proof []byte, period, index uint64, eb uint8, _ []byte) error {
	if len(proof) < 4 || !bytes.Equal(proof[:4], []byte("STWO")) {
		return fmt.Errorf("leanval: valset verify: not STWO")
	}
	if bytes.HasPrefix(proof, []byte("DSTW")) {
		return fmt.Errorf("leanval: valset verify: DummyStwo DSTW rejected")
	}
	if index >= (1 << 40) {
		return fmt.Errorf("leanval: valset AIR deposit index exceeds 5 bytes")
	}
	dir, err := leanStwoCrateDir()
	if err != nil {
		return err
	}
	cmd := exec.Command(
		"cargo", "run", "--offline", "--quiet", "--features", "real-stwo",
		"--bin", "lean-valset-air", "--",
		"verify", strconv.FormatUint(period, 10), indexToHex5(index), strconv.FormatUint(uint64(eb), 10),
	)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(proof)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("leanval: valset verify: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}
