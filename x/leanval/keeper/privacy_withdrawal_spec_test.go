package keeper

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// LEAN-7 hidden withdrawal spec only (no circuit). Commitment is H(addr, secret).
// Fail-closed until a withdrawal AIR exists.

type withdrawalSpecRow struct {
	addr   []byte
	secret []byte
}

func withdrawalCommitment(addr, secret []byte) [32]byte {
	h := sha256.New()
	h.Write(addr)
	h.Write(secret)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// UnimplementedWithdrawalAir is the fail-closed stub. Not a circuit.
type UnimplementedWithdrawalAir struct{}

func (UnimplementedWithdrawalAir) Commit(addr, secret []byte) ([32]byte, error) {
	return [32]byte{}, fmt.Errorf("not implemented")
}

func (UnimplementedWithdrawalAir) Open(commit [32]byte, addr, secret []byte) error {
	return fmt.Errorf("not implemented")
}

func TestPrivacy_WithdrawalCommitIsHashAddrSecret(t *testing.T) {
	air := HashAddrSecretWithdrawal{}
	row := withdrawalSpecRow{
		addr:   []byte("terp1hiddenwithdrawaddr00000000000"),
		secret: []byte("withdrawal-secret-32-bytes-pad!!"),
	}
	got, err := air.Commit(row.addr, row.secret)
	if err != nil {
		t.Fatalf("LEAN-7 withdrawal H(addr,secret) not implemented (red): %v", err)
	}
	want := withdrawalCommitment(row.addr, row.secret)
	if got != want {
		t.Fatalf("commit must be H(addr,secret)\n got %x\nwant %x", got, want)
	}
}

func TestPrivacy_SameAddrDifferentSecretDiffers(t *testing.T) {
	air := HashAddrSecretWithdrawal{}
	addr := []byte("terp1sameaddrxxxxxxxxxxxxxxxxxxxx")
	a, err := air.Commit(addr, []byte("secret-A"))
	if err != nil {
		t.Fatalf("LEAN-7 withdrawal not implemented (red): %v", err)
	}
	b, err := air.Commit(addr, []byte("secret-B"))
	if err != nil {
		t.Fatalf("LEAN-7 withdrawal not implemented (red): %v", err)
	}
	if a == b {
		t.Fatal("H(addr,secret) must change when secret changes")
	}
}

func TestPrivacy_OpenRequiresMatchingSecret(t *testing.T) {
	air := HashAddrSecretWithdrawal{}
	addr := []byte("terp1openaddr")
	secret := []byte("s1")
	want := withdrawalCommitment(addr, secret)
	if err := air.Open(want, addr, []byte("wrong")); err == nil {
		t.Fatal("open with wrong secret must fail")
	}
	if err := air.Open(want, addr, secret); err != nil {
		t.Fatalf("open with matching H(addr,secret) not implemented (red): %v", err)
	}
}

func TestPrivacy_StubRejectsUnimplemented(t *testing.T) {
	_, err := (UnimplementedWithdrawalAir{}).Commit([]byte("a"), []byte("b"))
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("UnimplementedWithdrawalAir must return not implemented, got %v", err)
	}
}

func TestPrivacy_SpecHashIsAddrThenSecret(t *testing.T) {
	addr := []byte{0x01, 0x02}
	secret := []byte{0x03, 0x04}
	got := withdrawalCommitment(addr, secret)
	h := sha256.Sum256([]byte{0x01, 0x02, 0x03, 0x04})
	if !bytes.Equal(got[:], h[:]) {
		t.Fatalf("spec hash is SHA256(addr||secret)")
	}
}

func TestPrivacy_SameSecretDifferentAddrDiffers(t *testing.T) {
	air := HashAddrSecretWithdrawal{}
	secret := []byte("shared-secret")
	a, err := air.Commit([]byte("addr-A"), secret)
	if err != nil {
		t.Fatalf("LEAN-7 withdrawal not implemented (red): %v", err)
	}
	b, err := air.Commit([]byte("addr-B"), secret)
	if err != nil {
		t.Fatalf("LEAN-7 withdrawal not implemented (red): %v", err)
	}
	if a == b {
		t.Fatal("H(addr,secret) must change when addr changes")
	}
}

func TestPrivacy_CommitDoesNotRevealAddr(t *testing.T) {
	air := HashAddrSecretWithdrawal{}
	addr := []byte("terp1hiddenwithdrawaddr00000000000")
	got, err := air.Commit(addr, []byte("s"))
	if err != nil {
		t.Fatalf("LEAN-7 withdrawal not implemented (red): %v", err)
	}
	if bytes.Contains(got[:], addr) {
		t.Fatal("commitment must not embed the withdrawal address")
	}
}

func TestPrivacy_HashOrderIsNotSecretThenAddr(t *testing.T) {
	air := HashAddrSecretWithdrawal{}
	addr := []byte{0x01, 0x02}
	secret := []byte{0x03, 0x04}
	got, err := air.Commit(addr, secret)
	if err != nil {
		t.Fatalf("LEAN-7 withdrawal not implemented (red): %v", err)
	}
	reversed := sha256.Sum256([]byte{0x03, 0x04, 0x01, 0x02})
	if got == reversed {
		t.Fatal("commit must be SHA256(addr||secret), not SHA256(secret||addr)")
	}
}
