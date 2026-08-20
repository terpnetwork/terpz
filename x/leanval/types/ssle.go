package types

import (
	"bytes"
	"encoding/binary"
)

// SSLEBlob is the proposer-injected hide-until-block object.
// Ticket is the scheduled view; Proposer is only valid after the block.
type SSLEBlob struct {
	Period   uint64
	Height   int64
	Ticket   []byte
	Proposer []byte
	Proof    []byte
}

// EncodeSSLE: SSLE | period(8) | height(8) | ticket(32) | plen(1) | proposer | proof
func EncodeSSLE(b SSLEBlob) []byte {
	plen := len(b.Proposer)
	if plen > 255 {
		plen = 255
		b.Proposer = b.Proposer[:255]
	}
	out := make([]byte, 0, 4+8+8+32+1+plen+len(b.Proof))
	out = append(out, PrefixSSLE...)
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], b.Period)
	binary.BigEndian.PutUint64(buf[8:16], uint64(b.Height))
	out = append(out, buf[:]...)
	t := b.Ticket
	if len(t) < 32 {
		pad := make([]byte, 32)
		copy(pad, t)
		t = pad
	}
	out = append(out, t[:32]...)
	out = append(out, byte(plen))
	out = append(out, b.Proposer[:plen]...)
	out = append(out, b.Proof...)
	return out
}

func DecodeSSLE(tx []byte) (SSLEBlob, bool) {
	const hdr = 4 + 8 + 8 + 32 + 1
	if !HasPrefix(tx, PrefixSSLE) || len(tx) < hdr {
		return SSLEBlob{}, false
	}
	period := binary.BigEndian.Uint64(tx[4:12])
	height := int64(binary.BigEndian.Uint64(tx[12:20]))
	ticket := append([]byte(nil), tx[20:52]...)
	plen := int(tx[52])
	if 53+plen > len(tx) {
		return SSLEBlob{}, false
	}
	proposer := append([]byte(nil), tx[53:53+plen]...)
	proof := append([]byte(nil), tx[53+plen:]...)
	return SSLEBlob{Period: period, Height: height, Ticket: ticket, Proposer: proposer, Proof: proof}, true
}

func RejectMempoolSSLE(tx []byte) error {
	if HasPrefix(tx, PrefixSSLE) {
		return ErrMempoolSSLE
	}
	return nil
}

func FindSSLE(txs [][]byte) (SSLEBlob, int, bool) {
	for i, tx := range txs {
		if blob, ok := DecodeSSLE(tx); ok {
			return blob, i, true
		}
	}
	return SSLEBlob{}, -1, false
}

func IsSSLEProofSTWO(proof []byte) bool {
	return len(proof) >= 10 && bytes.Equal(proof[:4], []byte("STWO")) && bytes.Equal(proof[6:10], []byte("SSLE"))
}
