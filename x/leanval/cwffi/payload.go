package cwffi

import (
	"encoding/binary"
	"fmt"
)

const (
	payloadMagic   = "LNCW"
	payloadVersion = 1
)

// Payload is the opaque simplex blob: height + txs (LNPR/JOIN/LEAV ride inside).
type Payload struct {
	Height int64
	Txs    [][]byte
}

func (p Payload) Encode() []byte {
	n := 4 + 1 + 8 + 4
	for _, tx := range p.Txs {
		n += 4 + len(tx)
	}
	out := make([]byte, 0, n)
	out = append(out, payloadMagic...)
	out = append(out, payloadVersion)
	var h [8]byte
	binary.BigEndian.PutUint64(h[:], uint64(p.Height))
	out = append(out, h[:]...)
	var c [4]byte
	binary.BigEndian.PutUint32(c[:], uint32(len(p.Txs)))
	out = append(out, c[:]...)
	for _, tx := range p.Txs {
		binary.BigEndian.PutUint32(c[:], uint32(len(tx)))
		out = append(out, c[:]...)
		out = append(out, tx...)
	}
	return out
}

func DecodePayload(b []byte) (Payload, error) {
	if len(b) < 4+1+8+4 {
		return Payload{}, fmt.Errorf("lean-cw: payload truncated")
	}
	if string(b[:4]) != payloadMagic {
		return Payload{}, fmt.Errorf("lean-cw: bad magic")
	}
	if b[4] != payloadVersion {
		return Payload{}, fmt.Errorf("lean-cw: bad version %d", b[4])
	}
	height := int64(binary.BigEndian.Uint64(b[5:13]))
	n := int(binary.BigEndian.Uint32(b[13:17]))
	i := 17
	txs := make([][]byte, 0, n)
	for k := 0; k < n; k++ {
		if i+4 > len(b) {
			return Payload{}, fmt.Errorf("lean-cw: payload truncated")
		}
		ln := int(binary.BigEndian.Uint32(b[i : i+4]))
		i += 4
		if i+ln > len(b) {
			return Payload{}, fmt.Errorf("lean-cw: payload truncated")
		}
		txs = append(txs, append([]byte(nil), b[i:i+ln]...))
		i += ln
	}
	return Payload{Height: height, Txs: txs}, nil
}
