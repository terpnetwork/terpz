//! Consensus-object **aggregate** (`loop_id=aggregate`).
//!
//! Cited: `RESEARCH-PACK-CONSENSUS-OBJECT.md` **Phase 1B** + rec #2:
//! Jiang same-statement compose-before-FRI. Named host Stwo `prover_id=2` / M31
//! (`curve_id=5`). Never `CircuitType::Stark`. Never Dummy `3a+5b+7`.
//! Never SHA256-RLC / FRIC counters as the verify.
//!
//! N same-statement instances → **one** in-process M31 FRI verify.
//! Does not replace Comet CheckTx/mempool.

mod fri;
mod m31;

use fri::{compose_evals, composed_coeffs, merkle_commit, prove, verify, MAGIC};

/// Matches `CircuitType::Stwo` (PROVER-IDS.md). Never a generic Stark id.
pub const CIRCUIT_TYPE_STWO: u8 = 2;
/// M31 / Circle field id.
pub const CURVE_TYPE_M31: u8 = 5;

/// Last-finalized period object roots + participation bitfield (Jiang PIs).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PeriodObjectRoots {
    pub period: u64,
    pub bitfield: [u8; 32],
    pub object_root: [u8; 32],
}

/// One same-statement leaf (period/vote proof to compose).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SameStatementLeaf {
    pub roots: PeriodObjectRoots,
}

/// Named Stwo/M31 aggregate: composed poly + one FRI proof.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct AggregateBlob {
    pub prover_id: u8,
    pub curve_id: u8,
    pub n_leaves: u16,
    pub period: u64,
    pub bitfield_or: [u8; 32],
    pub object_root: [u8; 32],
    /// Merkle root of the RS/FRI layer-0 codeword (not SHA256 of PIs).
    pub composed_commit: [u8; 32],
    /// Low-degree composed M31 coefficients (degree < 4).
    pub evals: Vec<u32>,
    pub fri: FriWire,
}

/// Opaque FRI transcript (queries + layer roots).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FriWire {
    pub roots: Vec<[u8; 32]>,
    pub n_queries: u8,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum AggError {
    Empty,
    DummyChecksum,
    MixedStatement,
    /// Costume / missing host — not returned after a real fold.
    BlockedStwoHost,
    BadMagic,
    Truncated,
    FriFail,
}

impl std::fmt::Display for AggError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{self:?}")
    }
}
impl std::error::Error for AggError {}

fn leaf_tuple(l: &SameStatementLeaf) -> (u64, [u8; 32], [u8; 32]) {
    (l.roots.period, l.roots.bitfield, l.roots.object_root)
}

/// Compose N same-statement leaves, then FRI-prove the sum polynomial.
pub fn compose_same_statement(leaves: &[SameStatementLeaf]) -> Result<AggregateBlob, AggError> {
    if leaves.is_empty() {
        return Err(AggError::Empty);
    }
    if leaves.len() < 2 {
        return Err(AggError::Empty);
    }
    let period = leaves[0].roots.period;
    let object_root = leaves[0].roots.object_root;
    let mut bitfield_or = [0u8; 32];
    for leaf in leaves {
        if leaf.roots.period != period || leaf.roots.object_root != object_root {
            return Err(AggError::MixedStatement);
        }
        for i in 0..32 {
            bitfield_or[i] |= leaf.roots.bitfield[i];
        }
    }
    let tuples: Vec<_> = leaves.iter().map(leaf_tuple).collect();
    let ev = compose_evals(&tuples);
    let proof = prove(&ev);
    let commit = merkle_commit(&ev);
    let evals = composed_coeffs(&tuples);
    Ok(AggregateBlob {
        prover_id: CIRCUIT_TYPE_STWO,
        curve_id: CURVE_TYPE_M31,
        n_leaves: leaves.len() as u16,
        period,
        bitfield_or,
        object_root,
        composed_commit: commit,
        evals,
        fri: FriWire {
            roots: proof.layer_roots,
            n_queries: proof.queries.len() as u8,
        },
    })
}

/// One FRI verify of the composed polynomial against the leaves.
pub fn verify_aggregate(blob: &AggregateBlob, leaves: &[SameStatementLeaf]) -> Result<(), AggError> {
    if leaves.len() != blob.n_leaves as usize {
        return Err(AggError::FriFail);
    }
    if blob.prover_id != CIRCUIT_TYPE_STWO || blob.curve_id != CURVE_TYPE_M31 {
        return Err(AggError::BadMagic);
    }
    let tuples: Vec<_> = leaves.iter().map(leaf_tuple).collect();
    let ev = compose_evals(&tuples);
    if merkle_commit(&ev) != blob.composed_commit {
        return Err(AggError::FriFail);
    }
    let proof = prove(&ev);
    if !verify(&ev, &proof) {
        return Err(AggError::FriFail);
    }
    if proof.layer_roots != blob.fri.roots {
        return Err(AggError::FriFail);
    }
    Ok(())
}

pub fn encode_blob(blob: &AggregateBlob) -> Result<Vec<u8>, AggError> {
    let mut o = Vec::new();
    o.extend_from_slice(MAGIC);
    o.push(blob.prover_id);
    o.push(blob.curve_id);
    o.extend_from_slice(&blob.n_leaves.to_le_bytes());
    o.extend_from_slice(&blob.period.to_le_bytes());
    o.extend_from_slice(&blob.bitfield_or);
    o.extend_from_slice(&blob.object_root);
    o.extend_from_slice(&blob.composed_commit);
    o.push(blob.evals.len() as u8);
    for e in &blob.evals {
        o.extend_from_slice(&e.to_le_bytes());
    }
    o.push(blob.fri.roots.len() as u8);
    for r in &blob.fri.roots {
        o.extend_from_slice(r);
    }
    o.push(blob.fri.n_queries);
    Ok(o)
}

pub fn decode_blob(bytes: &[u8]) -> Result<AggregateBlob, AggError> {
    if bytes.starts_with(b"DSTW") {
        return Err(AggError::DummyChecksum);
    }
    if bytes.starts_with(b"FRIC") {
        return Err(AggError::BadMagic);
    }
    if bytes.len() < 4 + 2 + 2 + 8 + 32 + 32 + 32 {
        return Err(AggError::Truncated);
    }
    if &bytes[0..4] != MAGIC {
        return Err(AggError::BadMagic);
    }
    let prover_id = bytes[4];
    let curve_id = bytes[5];
    if prover_id != CIRCUIT_TYPE_STWO || curve_id != CURVE_TYPE_M31 {
        return Err(AggError::BadMagic);
    }
    let n_leaves = u16::from_le_bytes([bytes[6], bytes[7]]);
    let period = u64::from_le_bytes(bytes[8..16].try_into().unwrap());
    let mut bitfield_or = [0u8; 32];
    bitfield_or.copy_from_slice(&bytes[16..48]);
    let mut object_root = [0u8; 32];
    object_root.copy_from_slice(&bytes[48..80]);
    let mut composed_commit = [0u8; 32];
    composed_commit.copy_from_slice(&bytes[80..112]);
    let mut i = 112;
    if i >= bytes.len() {
        return Err(AggError::Truncated);
    }
    let ne = bytes[i] as usize;
    i += 1;
    let mut evals = Vec::with_capacity(ne);
    for _ in 0..ne {
        if i + 4 > bytes.len() {
            return Err(AggError::Truncated);
        }
        evals.push(u32::from_le_bytes(bytes[i..i + 4].try_into().unwrap()));
        i += 4;
    }
    if i >= bytes.len() {
        return Err(AggError::Truncated);
    }
    let nr = bytes[i] as usize;
    i += 1;
    let mut roots = Vec::with_capacity(nr);
    for _ in 0..nr {
        if i + 32 > bytes.len() {
            return Err(AggError::Truncated);
        }
        let mut r = [0u8; 32];
        r.copy_from_slice(&bytes[i..i + 32]);
        roots.push(r);
        i += 32;
    }
    if i >= bytes.len() {
        return Err(AggError::Truncated);
    }
    let n_queries = bytes[i];
    Ok(AggregateBlob {
        prover_id,
        curve_id,
        n_leaves,
        period,
        bitfield_or,
        object_root,
        composed_commit,
        evals,
        fri: FriWire { roots, n_queries },
    })
}

/// N Dummy DSTW blobs are never a valid aggregate (pack Phase 1B / rec §2).
pub fn reject_dummy_batch(proofs: &[Vec<u8>]) -> Result<(), AggError> {
    if proofs.iter().any(|p| p.starts_with(b"DSTW")) {
        return Err(AggError::DummyChecksum);
    }
    if proofs.is_empty() {
        return Err(AggError::Empty);
    }
    Ok(())
}

#[cfg(test)]
mod real_fri_tdd;

#[cfg(test)]
mod tests {
    use super::*;

    fn leaf(period: u64, bits: u8, root_byte: u8) -> SameStatementLeaf {
        let mut bitfield = [0u8; 32];
        bitfield[0] = bits;
        let mut object_root = [0u8; 32];
        object_root[0] = root_byte;
        SameStatementLeaf {
            roots: PeriodObjectRoots {
                period,
                bitfield,
                object_root,
            },
        }
    }

    #[test]
    fn dummy_dstw_batch_fails() {
        let dummy = b"DSTW\x02\x05".to_vec();
        assert_eq!(
            reject_dummy_batch(&[dummy.clone(), dummy]),
            Err(AggError::DummyChecksum)
        );
        assert_eq!(decode_blob(b"DSTW\x02\x05abcdefgh"), Err(AggError::DummyChecksum));
    }

    #[test]
    fn sha256_fric_costume_is_not_an_aggregate() {
        assert_eq!(decode_blob(b"FRIC\x02\x05"), Err(AggError::BadMagic));
    }

    #[test]
    fn mixed_period_or_root_rejected() {
        let leaves = vec![leaf(1, 1, 1), leaf(2, 1, 1)];
        assert_eq!(compose_same_statement(&leaves), Err(AggError::MixedStatement));
        let leaves = vec![leaf(1, 1, 1), leaf(1, 1, 2)];
        assert_eq!(compose_same_statement(&leaves), Err(AggError::MixedStatement));
    }

    #[test]
    fn empty_not_aggregate() {
        assert_eq!(compose_same_statement(&[]), Err(AggError::Empty));
    }

    #[test]
    fn n_compose_one_fri_verify() {
        let leaves = vec![leaf(7, 1, 9), leaf(7, 2, 9), leaf(7, 4, 9)];
        let blob = compose_same_statement(&leaves).unwrap();
        assert_eq!(blob.prover_id, CIRCUIT_TYPE_STWO);
        assert_eq!(blob.curve_id, CURVE_TYPE_M31);
        assert_eq!(blob.n_leaves, 3);
        verify_aggregate(&blob, &leaves).unwrap();
        let enc = encode_blob(&blob).unwrap();
        assert_eq!(&enc[0..4], b"STWO");
        assert!(!enc.starts_with(b"DSTW"));
        assert!(!enc.starts_with(b"FRIC"));
    }
}
