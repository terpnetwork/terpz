package types

// Store prefixes. Single KV tree — never LastValidatorPowers.
const (
	ModuleName = "leanval"
	StoreKey   = ModuleName

	// FlagOwnsValset: default off. When true, app EndBlocker overwrites ValidatorUpdates.
	FlagOwnsValset = "leanval_owns_valset"

	// BondedPrefix keys: BondedPrefix | period(8 BE) | subject
	BondedPrefix byte = 0x01
	// SubjectIndexPrefix keys: SubjectIndexPrefix | subject | period(8 BE)
	SubjectIndexPrefix byte = 0x02
	// LastUpdatesPrefix holds last emitted (subject → power) for diffs.
	LastUpdatesPrefix byte = 0x03
	// OwnsValsetPrefix persists leanval_owns_valset across restart (not RAM-only).
	OwnsValsetPrefix byte = 0x00
	// ObjectRootsPrefix: last committed deposit||bitfield||EB roots (32*3).
	// VerifyLNPR binds Dummy instances to this store value, not the tx.
	ObjectRootsPrefix byte = 0x04
)

// ObjectRootsSize is deposit(32) || bitfield(32) || eb(32).
const ObjectRootsSize = 96

// Inject prefixes (4-byte ASCII). Hashmerchant owns HMVE; we wrap, not replace.
var (
	PrefixHMVE = []byte("HMVE") // 0x48 0x4D 0x56 0x45
	PrefixLNPR = []byte("LNPR") // 0x4C 0x4E 0x50 0x52
)

// BlocksPerPeriod is the height stub for a 1-hour period.
// Documented N: 600 ≈ 1h at 6s block time. Real wall-clock hour can replace this
// without changing BondedSet(period) semantics.
const BlocksPerPeriod int64 = 600

// PeriodFromHeight maps height → period P. Height 0..N-1 is period 0.
// TestLeanVerifierAcc is 20 zero bytes (v1 hardcoded waist; consensus param later).
func TestLeanVerifierAcc() []byte { return make([]byte, 20) }

func OwnsValsetKey() []byte { return []byte{OwnsValsetPrefix} }

func ObjectRootsKey() []byte { return []byte{ObjectRootsPrefix} }

func PeriodFromHeight(height int64) uint64 {
	if height < 0 {
		return 0
	}
	return uint64(height / BlocksPerPeriod)
}

func BondedKey(period uint64, subject []byte) []byte {
	k := make([]byte, 1+8+len(subject))
	k[0] = BondedPrefix
	putU64(k[1:9], period)
	copy(k[9:], subject)
	return k
}

func BondedPrefixForPeriod(period uint64) []byte {
	k := make([]byte, 1+8)
	k[0] = BondedPrefix
	putU64(k[1:9], period)
	return k
}

func LastPowerKey(subject []byte) []byte {
	k := make([]byte, 1+len(subject))
	k[0] = LastUpdatesPrefix
	copy(k[1:], subject)
	return k
}

func putU64(dst []byte, v uint64) {
	dst[0] = byte(v >> 56)
	dst[1] = byte(v >> 48)
	dst[2] = byte(v >> 40)
	dst[3] = byte(v >> 32)
	dst[4] = byte(v >> 24)
	dst[5] = byte(v >> 16)
	dst[6] = byte(v >> 8)
	dst[7] = byte(v)
}

func GetU64(src []byte) uint64 {
	if len(src) < 8 {
		return 0
	}
	return uint64(src[0])<<56 | uint64(src[1])<<48 | uint64(src[2])<<40 | uint64(src[3])<<32 |
		uint64(src[4])<<24 | uint64(src[5])<<16 | uint64(src[6])<<8 | uint64(src[7])
}

func PutI64(v int64) []byte {
	b := make([]byte, 8)
	putU64(b, uint64(v))
	return b
}

func GetI64(src []byte) int64 {
	return int64(GetU64(src))
}
