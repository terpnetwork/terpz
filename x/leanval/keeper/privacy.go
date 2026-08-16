package keeper

import (
	"crypto/sha256"
	"fmt"
)

// HashAddrSecretWithdrawal is LEAN-7 hidden withdrawal (no circuit).
// Commitment is SHA256(addr || secret). Fail-closed Open on mismatch.
type HashAddrSecretWithdrawal struct{}

func (HashAddrSecretWithdrawal) Commit(addr, secret []byte) ([32]byte, error) {
	return hashAddrSecret(addr, secret), nil
}

func (HashAddrSecretWithdrawal) Open(commit [32]byte, addr, secret []byte) error {
	got := hashAddrSecret(addr, secret)
	if got != commit {
		return fmt.Errorf("leanval: withdrawal open mismatch")
	}
	return nil
}

func hashAddrSecret(addr, secret []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write(addr)
	_, _ = h.Write(secret)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
