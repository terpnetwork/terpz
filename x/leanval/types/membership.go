package types

import (
	"fmt"

	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// JOIN / LEAV are mempool-legal membership txs (not inject-class).
// LNPR stays proposer-inject only. Disk lean-pending.json is not admission.
var (
	PrefixJOIN = []byte("JOIN")
	PrefixLEAV = []byte("LEAV")
)

const MembershipVersion byte = 1

// JoinBlob is a committed join request: next Prepare includes the subject
// because BondedSet already has the row.
type JoinBlob struct {
	Period  uint64
	Subject []byte
	Weight  int64
	Proof   []byte // valset STWO extra; Dummy DSTW must not appear
}

// LeaveBlob is a committed leave: BondedSet drops the subject.
type LeaveBlob struct {
	Period  uint64
	Subject []byte
}

func EncodeJoin(b JoinBlob) []byte {
	out := append([]byte{}, PrefixJOIN...)
	out = append(out, MembershipVersion)
	per := make([]byte, 8)
	putU64(per, b.Period)
	out = append(out, per...)
	subj := b.Subject
	if len(subj) > 255 {
		subj = subj[:255]
	}
	out = append(out, byte(len(subj)))
	out = append(out, subj...)
	out = append(out, PutI64(b.Weight)...)
	if len(b.Proof) > 0 {
		out = append(out, b.Proof...)
	}
	return out
}

func DecodeJoin(tx []byte) (JoinBlob, bool) {
	var z JoinBlob
	if !HasPrefix(tx, PrefixJOIN) {
		return z, false
	}
	p := tx[len(PrefixJOIN):]
	if len(p) < 1+8+1 || p[0] != MembershipVersion {
		return z, false
	}
	p = p[1:]
	z.Period = GetU64(p[:8])
	p = p[8:]
	al := int(p[0])
	p = p[1:]
	if len(p) < al+8 {
		return z, false
	}
	z.Subject = append([]byte(nil), p[:al]...)
	z.Weight = GetI64(p[al : al+8])
	if len(p) > al+8 {
		z.Proof = append([]byte(nil), p[al+8:]...)
	}
	return z, true
}

func EncodeLeave(b LeaveBlob) []byte {
	out := append([]byte{}, PrefixLEAV...)
	out = append(out, MembershipVersion)
	per := make([]byte, 8)
	putU64(per, b.Period)
	out = append(out, per...)
	subj := b.Subject
	if len(subj) > 255 {
		subj = subj[:255]
	}
	out = append(out, byte(len(subj)))
	out = append(out, subj...)
	return out
}

func DecodeLeave(tx []byte) (LeaveBlob, bool) {
	var z LeaveBlob
	if !HasPrefix(tx, PrefixLEAV) {
		return z, false
	}
	p := tx[len(PrefixLEAV):]
	if len(p) < 1+8+1 || p[0] != MembershipVersion {
		return z, false
	}
	p = p[1:]
	z.Period = GetU64(p[:8])
	p = p[8:]
	al := int(p[0])
	p = p[1:]
	if len(p) < al {
		return z, false
	}
	z.Subject = append([]byte(nil), p[:al]...)
	return z, true
}

func IsMembershipTx(tx []byte) bool {
	return HasPrefix(tx, PrefixJOIN) || HasPrefix(tx, PrefixLEAV)
}

// MembershipTx is a decoder stub so JOIN/LEAV can CheckTx without Amino/proto.
// Ante must skip this type (no signatures). PreBlock applies the raw bytes.
type MembershipTx struct {
	Raw []byte
}

func (t MembershipTx) GetMsgs() []sdk.Msg { return nil }

func (t MembershipTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func (t MembershipTx) GetGas() uint64 { return 0 }

func (t MembershipTx) GetFee() sdk.Coins { return nil }

func (t MembershipTx) FeePayer() []byte { return nil }

func (t MembershipTx) FeeGranter() []byte { return nil }

func (t MembershipTx) GetMemo() string { return "" }

func ValidateJoin(b JoinBlob) error {
	if len(b.Subject) == 0 {
		return fmt.Errorf("leanval: join subject empty")
	}
	if b.Weight <= 0 {
		return fmt.Errorf("leanval: join weight must be > 0")
	}
	return nil
}

func ValidateLeave(b LeaveBlob) error {
	if len(b.Subject) == 0 {
		return fmt.Errorf("leanval: leave subject empty")
	}
	return nil
}
