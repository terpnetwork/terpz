package types

import "bytes"

const LNPRVersion byte = 1

// HasPrefix reports whether tx starts with p.
func HasPrefix(tx, p []byte) bool {
	return bytes.HasPrefix(tx, p)
}

// LNPRBlob is the injected proposal tx (not a user Msg).
//
//	LNPR | version(1) | period(8 BE) | nsubj(2 BE) | repeated {
//	  addrLen(1) | addr | weight(8 BE) | proofLen(4 BE) | proof
//	}
type LNPRBlob struct {
	Period   uint64
	Subjects []SubjectProof
}

type SubjectProof struct {
	Subject []byte
	Weight  int64
	Proof   []byte
}

func EncodeLNPR(b LNPRBlob) []byte {
	out := append([]byte{}, PrefixLNPR...)
	out = append(out, LNPRVersion)
	per := make([]byte, 8)
	putU64(per, b.Period)
	out = append(out, per...)
	n := len(b.Subjects)
	out = append(out, byte(n>>8), byte(n))
	for _, s := range b.Subjects {
		if len(s.Subject) > 255 {
			s.Subject = s.Subject[:255]
		}
		out = append(out, byte(len(s.Subject)))
		out = append(out, s.Subject...)
		out = append(out, PutI64(s.Weight)...)
		pl := len(s.Proof)
		out = append(out, byte(pl>>24), byte(pl>>16), byte(pl>>8), byte(pl))
		out = append(out, s.Proof...)
	}
	return out
}

func DecodeLNPR(tx []byte) (LNPRBlob, bool) {
	var z LNPRBlob
	if !HasPrefix(tx, PrefixLNPR) {
		return z, false
	}
	p := tx[len(PrefixLNPR):]
	if len(p) < 1+8+2 || p[0] != LNPRVersion {
		return z, false
	}
	p = p[1:]
	z.Period = GetU64(p[:8])
	p = p[8:]
	n := int(p[0])<<8 | int(p[1])
	p = p[2:]
	for i := 0; i < n; i++ {
		if len(p) < 1 {
			return z, false
		}
		al := int(p[0])
		p = p[1:]
		if len(p) < al+8+4 {
			return z, false
		}
		subj := append([]byte(nil), p[:al]...)
		p = p[al:]
		w := GetI64(p[:8])
		p = p[8:]
		pl := int(p[0])<<24 | int(p[1])<<16 | int(p[2])<<8 | int(p[3])
		p = p[4:]
		if pl < 0 || len(p) < pl {
			return z, false
		}
		proof := append([]byte(nil), p[:pl]...)
		p = p[pl:]
		z.Subjects = append(z.Subjects, SubjectProof{Subject: subj, Weight: w, Proof: proof})
	}
	return z, true
}

// MaxProofBytes is the host cap (DUMMY-STWO). Larger → invalid, no verify.
const MaxProofBytes = 2 << 20

// MaxRawStarksPerLNPR is LEAN-5: one slot must not ship a raw STARK per
// bonded subject. Fold covers the roster via object-root inclusion; extras
// (JOIN) may carry individual STWO proofs up to this cap.
const MaxRawStarksPerLNPR = 8

// TxDecoder is the subset of sdk.TxDecoder we wrap (avoid sdk import in types).
type TxDecoder func([]byte) (any, error)

// RejectMempoolLNPR is the vote-sdk CheckTx pattern: inject-class bytes never enter the mempool.
func RejectMempoolLNPR(tx []byte) error {
	if HasPrefix(tx, PrefixLNPR) {
		return ErrMempoolLNPR
	}
	return nil
}
