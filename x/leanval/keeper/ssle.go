package keeper

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// HideUntilBlockSSLE is LEAN-8 schedule-only SSLE (no circuit).
// The scheduled view is a ticket; the proposer id is available only after
// the block that elects it is supplied.
type HideUntilBlockSSLE struct{}

func (HideUntilBlockSSLE) HiddenProposerID(period uint64, height int64) ([]byte, error) {
	if height < 0 {
		return nil, fmt.Errorf("leanval: ssle height")
	}
	return ssleTicket(period, height), nil
}

func (HideUntilBlockSSLE) RevealProposerAfterBlock(period uint64, height int64, block []byte) ([]byte, error) {
	if height < 0 {
		return nil, fmt.Errorf("leanval: ssle height")
	}
	if len(block) == 0 {
		return nil, fmt.Errorf("leanval: ssle missing block")
	}
	return ssleReveal(period, height, block), nil
}

func ssleTicket(period uint64, height int64) []byte {
	var buf [8 + 8]byte
	binary.BigEndian.PutUint64(buf[0:8], period)
	binary.BigEndian.PutUint64(buf[8:16], uint64(height))
	sum := sha256.Sum256(append([]byte("leanval/ssle/ticket/v1"), buf[:]...))
	return sum[:]
}

func ssleReveal(period uint64, height int64, block []byte) []byte {
	var buf [8 + 8]byte
	binary.BigEndian.PutUint64(buf[0:8], period)
	binary.BigEndian.PutUint64(buf[8:16], uint64(height))
	h := sha256.New()
	_, _ = h.Write([]byte("leanval/ssle/reveal/v1"))
	_, _ = h.Write(buf[:])
	_, _ = h.Write(block)
	return h.Sum(nil)
}
