//! Same-statement fold (RESEARCH-PACK Phase 1B / Jiang): public inputs are
//! bitfield root + deposit root + EB root. Not N Dummy (a,b) pairs and not
//! c=3a+5b+7 over a subject roster.

use itertools::Itertools;
use num_traits::Zero;
use stwo::core::channel::{Blake2sChannel, Channel};
use stwo::core::fields::m31::{BaseField, M31 as StwoM31};
use stwo::core::fields::qm31::QM31;
use stwo::core::pcs::{CommitmentSchemeVerifier, PcsConfig};
use stwo::core::poly::circle::CanonicCoset;
use stwo::core::proof::StarkProof;
use stwo::core::vcs_lifted::blake2_merkle::{Blake2sMerkleChannel, Blake2sMerkleHasher};
use stwo::core::verifier::verify;
use stwo::prover::backend::simd::SimdBackend;
use stwo::prover::backend::simd::column::BaseColumn;
use stwo::prover::backend::simd::m31::LOG_N_LANES;
use stwo::prover::poly::BitReversedOrder;
use stwo::prover::poly::circle::{CircleEvaluation, PolyOps};
use stwo::prover::{CommitmentSchemeProver, prove};
use stwo_constraint_framework::{
    EvalAtRow, FrameworkComponent, FrameworkEval, InfoEvaluator, PREPROCESSED_TRACE_IDX,
    TraceLocationAllocator,
};

use crate::{
    dummy_m31_hash, CIRCUIT_TYPE_STWO, CURVE_TYPE_M31, M31, M31_P, MAX_PROOF_BYTES, STWO_MAGIC,
    VerifyError,
};

/// Wire kind: same-statement fold over object roots.
pub const FOLD_KIND: &[u8; 4] = b"FOLD";

pub const ROOT_LEN: usize = 32;
pub const OBJECT_ROOTS_LEN: usize = ROOT_LEN * 3;

type HashComponent = FrameworkComponent<HashEval>;

#[derive(Clone)]
struct HashEval {
    log_n_rows: u32,
}

impl FrameworkEval for HashEval {
    fn log_size(&self) -> u32 {
        self.log_n_rows
    }

    fn max_constraint_log_degree_bound(&self) -> u32 {
        self.log_n_rows + 1
    }

    fn evaluate<E: EvalAtRow>(&self, mut eval: E) -> E {
        let a = eval.next_trace_mask();
        let b = eval.next_trace_mask();
        let c = eval.next_trace_mask();
        let three = E::F::from(BaseField::from_u32_unchecked(3));
        let five = E::F::from(BaseField::from_u32_unchecked(5));
        let seven = E::F::from(BaseField::from_u32_unchecked(7));
        eval.add_constraint(c - a * three - b * five - seven);
        eval
    }
}

const LOG_N_ROWS: u32 = LOG_N_LANES;

/// Fold 32 bytes into one M31 (not a cryptographic hash; PI bind only).
pub fn root_m31(root: &[u8; ROOT_LEN]) -> M31 {
    let mut acc: u64 = 0;
    for chunk in root.chunks_exact(4) {
        let v = u32::from_le_bytes(chunk.try_into().unwrap()) as u64;
        acc = (acc.wrapping_mul(251).wrapping_add(v)) % (M31_P as u64);
    }
    M31(acc as u32)
}

/// Mix deposit||eb so the dummy AIR still has two seeds + claimed hash.
fn seeds(bitfield: &[u8; 32], deposit: &[u8; 32], eb: &[u8; 32]) -> (M31, M31, M31) {
    let a = root_m31(bitfield);
    let b = M31(((root_m31(deposit).0 as u64 * 17 + root_m31(eb).0 as u64) % (M31_P as u64)) as u32);
    let c = dummy_m31_hash(a, b);
    (a, b, c)
}

fn log_sizes() -> stwo::core::pcs::TreeVec<Vec<u32>> {
    let info = HashEval {
        log_n_rows: LOG_N_ROWS,
    }
    .evaluate(InfoEvaluator::empty());
    let mut sizes = info.mask_offsets.as_cols_ref().map_cols(|_| LOG_N_ROWS);
    sizes[PREPROCESSED_TRACE_IDX] = vec![];
    sizes
}

fn gen_trace(
    a: M31,
    b: M31,
    c: M31,
) -> Vec<CircleEvaluation<SimdBackend, StwoM31, BitReversedOrder>> {
    let n = 1usize << LOG_N_ROWS;
    let domain = CanonicCoset::new(LOG_N_ROWS).circle_domain();
    let z = StwoM31::from_u32_unchecked(0);
    let pad_c = StwoM31::from_u32_unchecked(dummy_m31_hash(M31(0), M31(0)).0);
    let mut col_a = vec![z; n];
    let mut col_b = vec![z; n];
    let mut col_c = vec![pad_c; n];
    col_a[0] = StwoM31::from_u32_unchecked(a.0);
    col_b[0] = StwoM31::from_u32_unchecked(b.0);
    col_c[0] = StwoM31::from_u32_unchecked(c.0);
    [col_a, col_b, col_c]
        .into_iter()
        .map(|col| {
            CircleEvaluation::<SimdBackend, _, BitReversedOrder>::new(
                domain,
                BaseColumn::from_iter(col),
            )
        })
        .collect_vec()
}

fn mix_public(channel: &mut Blake2sChannel, roots: &[u8; OBJECT_ROOTS_LEN], a: M31, b: M31, c: M31) {
    for chunk in roots.chunks_exact(8) {
        let mut buf = [0u8; 8];
        buf.copy_from_slice(chunk);
        channel.mix_u64(u64::from_le_bytes(buf));
    }
    channel.mix_u64(a.0 as u64);
    channel.mix_u64(b.0 as u64);
    channel.mix_u64(c.0 as u64);
}

fn header(roots: &[u8; OBJECT_ROOTS_LEN]) -> Vec<u8> {
    let mut out = Vec::with_capacity(10 + OBJECT_ROOTS_LEN);
    out.extend_from_slice(STWO_MAGIC);
    out.push(CIRCUIT_TYPE_STWO);
    out.push(CURVE_TYPE_M31);
    out.extend_from_slice(FOLD_KIND);
    out.extend_from_slice(roots);
    out
}

fn parse_header(proof: &[u8]) -> Result<([u8; OBJECT_ROOTS_LEN], usize), VerifyError> {
    if proof.len() > MAX_PROOF_BYTES {
        return Err(VerifyError::ProofTooLarge { len: proof.len() });
    }
    let hdr = 10 + OBJECT_ROOTS_LEN;
    if proof.len() < hdr {
        return Err(VerifyError::Truncated);
    }
    if &proof[0..4] != STWO_MAGIC {
        return Err(VerifyError::BadMagic);
    }
    if proof.windows(4).any(|w| w == b"DSTW") {
        return Err(VerifyError::BadMagic);
    }
    let prover_id = proof[4];
    let curve_id = proof[5];
    match crate::CircuitType::from_u8(prover_id) {
        Ok(crate::CircuitType::Stwo) => {}
        Ok(_) => return Err(VerifyError::WrongProverId { got: prover_id }),
        Err(e) => return Err(e),
    }
    match crate::CurveType::from_u8(curve_id) {
        Ok(crate::CurveType::M31) => {}
        Ok(_) => return Err(VerifyError::WrongCurveId { got: curve_id }),
        Err(e) => return Err(e),
    }
    if &proof[6..10] != FOLD_KIND {
        return Err(VerifyError::BadMagic);
    }
    let mut roots = [0u8; OBJECT_ROOTS_LEN];
    roots.copy_from_slice(&proof[10..hdr]);
    Ok((roots, hdr))
}

pub fn prove_fold(bitfield: [u8; 32], deposit: [u8; 32], eb: [u8; 32]) -> Result<Vec<u8>, VerifyError> {
    let mut roots = [0u8; OBJECT_ROOTS_LEN];
    roots[..32].copy_from_slice(&bitfield);
    roots[32..64].copy_from_slice(&deposit);
    roots[64..96].copy_from_slice(&eb);
    let (a, b, c) = seeds(&bitfield, &deposit, &eb);
    let config = PcsConfig::default();
    let twiddles = SimdBackend::precompute_twiddles(
        CanonicCoset::new(LOG_N_ROWS + config.fri_config.log_blowup_factor + 1)
            .circle_domain()
            .half_coset,
    );
    let channel = &mut Blake2sChannel::default();
    config.mix_into(channel);
    let mut commitment_scheme =
        CommitmentSchemeProver::<_, Blake2sMerkleChannel>::new(config, &twiddles);

    let tree_builder = commitment_scheme.tree_builder();
    tree_builder.commit(channel);
    mix_public(channel, &roots, a, b, c);
    let mut tree_builder = commitment_scheme.tree_builder();
    tree_builder.extend_evals(gen_trace(a, b, c));
    tree_builder.commit(channel);

    let component = HashComponent::new(
        &mut TraceLocationAllocator::default(),
        HashEval {
            log_n_rows: LOG_N_ROWS,
        },
        QM31::zero(),
    );

    let stark_proof =
        prove(&[&component], channel, commitment_scheme).map_err(|_| VerifyError::StwoVerify)?;
    let body = bincode::serialize(&stark_proof).map_err(|_| VerifyError::StwoVerify)?;
    let mut out = header(&roots);
    out.extend_from_slice(&body);
    if out.len() > MAX_PROOF_BYTES {
        return Err(VerifyError::ProofTooLarge { len: out.len() });
    }
    Ok(out)
}

pub fn verify_fold(
    proof: &[u8],
    bitfield: [u8; 32],
    deposit: [u8; 32],
    eb: [u8; 32],
) -> Result<(), VerifyError> {
    let (got, hdr) = parse_header(proof)?;
    if got[..32] != bitfield || got[32..64] != deposit || got[64..96] != eb {
        return Err(VerifyError::PublicInputMismatch);
    }
    let (a, b, c) = seeds(&bitfield, &deposit, &eb);
    let stark_proof: StarkProof<Blake2sMerkleHasher> =
        bincode::deserialize(&proof[hdr..]).map_err(|_| VerifyError::StwoVerify)?;

    let pcs_config = stark_proof.config;
    let channel = &mut Blake2sChannel::default();
    pcs_config.mix_into(channel);
    let commitment_scheme = &mut CommitmentSchemeVerifier::<Blake2sMerkleChannel>::new(pcs_config);
    let sizes = log_sizes();
    commitment_scheme.commit(stark_proof.commitments[0], &sizes[0], channel);
    mix_public(channel, &got, a, b, c);
    commitment_scheme.commit(stark_proof.commitments[1], &sizes[1], channel);

    let component = HashComponent::new(
        &mut TraceLocationAllocator::default(),
        HashEval {
            log_n_rows: LOG_N_ROWS,
        },
        QM31::zero(),
    );
    verify(&[&component], channel, commitment_scheme, stark_proof)
        .map_err(|_| VerifyError::StwoVerify)
}

/// Reject Dummy-N (concatenated DSTW) as an aggregate. Always Err.
pub fn reject_dummy_n(blob: &[u8]) -> Result<(), VerifyError> {
    if blob.windows(4).any(|w| w == b"DSTW") {
        return Err(VerifyError::BadMagic);
    }
    Err(VerifyError::StwoVerify)
}
