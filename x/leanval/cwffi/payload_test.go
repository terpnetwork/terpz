package cwffi

import (
	"bytes"
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestPayloadRoundtripLNPRJoin(t *testing.T) {
	lnpr := types.EncodeLNPR(types.LNPRBlob{Period: 1})
	join := append(append([]byte{}, types.PrefixJOIN...), 0x01)
	p := Payload{Height: 3, Txs: [][]byte{lnpr, join}}
	got, err := DecodePayload(p.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if got.Height != 3 || len(got.Txs) != 2 {
		t.Fatalf("%+v", got)
	}
	if !types.HasPrefix(got.Txs[0], types.PrefixLNPR) {
		t.Fatal("LNPR must stay LNPR")
	}
	if !types.HasPrefix(got.Txs[1], types.PrefixJOIN) {
		t.Fatal("JOIN stays a mempool tx, admitted via LNPR")
	}
}

func TestPayloadRejectsTruncated(t *testing.T) {
	if _, err := DecodePayload([]byte("LNC")); err == nil {
		t.Fatal("truncated")
	}
}

func TestDummyDSTWBytesUnchanged(t *testing.T) {
	proof := []byte{'D', 'S', 'T', 'W', 2, 5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	p := Payload{Height: 2, Txs: [][]byte{proof}}
	got, err := DecodePayload(p.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Txs[0], proof) {
		t.Fatalf("%x", got.Txs[0])
	}
	if got.Txs[0][4] != 2 || got.Txs[0][5] != 5 {
		t.Fatal("prover_id=2 curve_id=5")
	}
}
