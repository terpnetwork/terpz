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
	// PendingJoinPrefix: period(8) | subject → weight. Survives LNPR replace-set.
	PendingJoinPrefix byte = 0x05
	// PendingLeavePrefix: period(8) | subject.
	PendingLeavePrefix byte = 0x06
	// BitfieldPrefix: participation bits (OR-merge). SOURCES.md Phase 1B.
	BitfieldPrefix byte = 0x10
	// DepositIndexPrefix: subject -> 5-byte deposit-tree index. SOURCES.md Phase 1A.
	DepositIndexPrefix byte = 0x11
	// NextDepositIndexPrefix: u32 next index to allocate on JOIN.
	NextDepositIndexPrefix byte = 0x12
	// EBPrefix: deposit index -> 1-byte effective balance (separate tree). SOURCES 1B.
	EBPrefix byte = 0x13
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

func PendingJoinKey(period uint64, subject []byte) []byte {
	k := make([]byte, 1+8+len(subject))
	k[0] = PendingJoinPrefix
	putU64(k[1:9], period)
	copy(k[9:], subject)
	return k
}

func PendingJoinPrefixForPeriod(period uint64) []byte {
	k := make([]byte, 1+8)
	k[0] = PendingJoinPrefix
	putU64(k[1:9], period)
	return k
}

func PendingLeaveKey(period uint64, subject []byte) []byte {
	k := make([]byte, 1+8+len(subject))
	k[0] = PendingLeavePrefix
	putU64(k[1:9], period)
	copy(k[9:], subject)
	return k
}

func BitfieldKey() []byte { return []byte{BitfieldPrefix} }

func NextDepositIndexKey() []byte { return []byte{NextDepositIndexPrefix} }

func DepositIndexKey(subject []byte) []byte {
	k := make([]byte, 1+len(subject))
	k[0] = DepositIndexPrefix
	copy(k[1:], subject)
	return k
}

func EBKey(index uint32) []byte {
	k := make([]byte, 1+4)
	k[0] = EBPrefix
	k[1] = byte(index >> 24)
	k[2] = byte(index >> 16)
	k[3] = byte(index >> 8)
	k[4] = byte(index)
	return k
}

func PutU32(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func GetU32(src []byte) uint32 {
	if len(src) < 4 {
		return 0
	}
	return uint32(src[0])<<24 | uint32(src[1])<<16 | uint32(src[2])<<8 | uint32(src[3])
}

func DepositIndexBytes(index uint32) []byte {
	// ~5 bytes: 1 reserved + u32 (SOURCES 1A: deposit-tree INDEX not 32-byte subject).
	return append([]byte{0}, PutU32(index)...)
}

func PendingLeavePrefixForPeriod(period uint64) []byte {
	k := make([]byte, 1+8)
	k[0] = PendingLeavePrefix
	putU64(k[1:9], period)
	return k
}

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
