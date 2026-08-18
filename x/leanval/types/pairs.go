package types

import "fmt"

// DecodeBondedSubspace unpacks an IAVL /store/leanval/subspace query
// (cosmos store kv.Pairs proto: repeated Pair{key=1,value=2}).
func DecodeBondedSubspace(period uint64, bz []byte) ([]BondedSetRow, error) {
	if len(bz) == 0 {
		return nil, nil
	}
	pairs, err := decodeKVPairs(bz)
	if err != nil {
		return nil, fmt.Errorf("leanval subspace: %w", err)
	}
	pref := BondedPrefixForPeriod(period)
	out := make([]BondedSetRow, 0, len(pairs))
	for _, p := range pairs {
		if len(p.key) < len(pref) {
			continue
		}
		subj := append([]byte(nil), p.key[len(pref):]...)
		row := BondedSetRow{Subject: subj}
		if len(p.value) >= 9 {
			row.HasProof = p.value[0] == 1
			if row.HasProof {
				row.Weight = GetI64(p.value[1:9])
			}
		}
		out = append(out, row)
	}
	return out, nil
}

type rawKV struct{ key, value []byte }

func decodeKVPairs(bz []byte) ([]rawKV, error) {
	var out []rawKV
	i := 0
	for i < len(bz) {
		tag, n := consumeVarintBytes(bz[i:])
		if n <= 0 {
			return nil, fmt.Errorf("bad tag at %d", i)
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if fn != 1 || wt != 2 {
			return nil, fmt.Errorf("expected pairs field 1 length-delimited, got fn=%d wt=%d", fn, wt)
		}
		ln, n := consumeVarintBytes(bz[i:])
		if n <= 0 {
			return nil, fmt.Errorf("bad pair len")
		}
		i += n
		if i+int(ln) > len(bz) {
			return nil, fmt.Errorf("pair overflow")
		}
		p, err := decodePair(bz[i : i+int(ln)])
		if err != nil {
			return nil, err
		}
		out = append(out, p)
		i += int(ln)
	}
	return out, nil
}

func decodePair(bz []byte) (rawKV, error) {
	var p rawKV
	i := 0
	for i < len(bz) {
		tag, n := consumeVarintBytes(bz[i:])
		if n <= 0 {
			return p, fmt.Errorf("bad pair tag")
		}
		i += n
		fn, wt := int(tag>>3), int(tag&7)
		if wt != 2 {
			return p, fmt.Errorf("pair field %d not bytes", fn)
		}
		ln, n := consumeVarintBytes(bz[i:])
		if n <= 0 || i+n+int(ln) > len(bz) {
			return p, fmt.Errorf("pair field overflow")
		}
		i += n
		val := append([]byte(nil), bz[i:i+int(ln)]...)
		i += int(ln)
		switch fn {
		case 1:
			p.key = val
		case 2:
			p.value = val
		}
	}
	return p, nil
}

func consumeVarintBytes(b []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, c := range b {
		if c < 0x80 {
			return x | uint64(c)<<s, i + 1
		}
		x |= uint64(c&0x7f) << s
		s += 7
		if i > 9 {
			return 0, 0
		}
	}
	return 0, 0
}

// encodeKVPairs is test-only wire for DecodeBondedSubspace.
func encodeKVPairs(pairs []rawKV) []byte {
	var out []byte
	for _, p := range pairs {
		inner := append(protoBytes(1, p.key), protoBytes(2, p.value)...)
		out = append(out, protoBytes(1, inner)...)
	}
	return out
}

func protoBytes(fn int, v []byte) []byte {
	tag := appendUvarint(nil, uint64(fn<<3|2))
	tag = appendUvarint(tag, uint64(len(v)))
	return append(tag, v...)
}

func appendUvarint(b []byte, x uint64) []byte {
	for x >= 0x80 {
		b = append(b, byte(x)|0x80)
		x >>= 7
	}
	return append(b, byte(x))
}

// KV is one IAVL pair from /store/leanval/subspace.
type KV struct{ Key, Value []byte }

// DecodeSubspacePairs unpacks cosmos store kv.Pairs proto.
func DecodeSubspacePairs(bz []byte) ([]KV, error) {
	raw, err := decodeKVPairs(bz)
	if err != nil {
		return nil, err
	}
	out := make([]KV, len(raw))
	for i, p := range raw {
		out[i] = KV{Key: p.key, Value: p.value}
	}
	return out, nil
}

// CountSetBits counts 1-bits in a participation bitfield.
func CountSetBits(bf []byte) int {
	n := 0
	for _, b := range bf {
		for b != 0 {
			n++
			b &= b - 1
		}
	}
	return n
}
