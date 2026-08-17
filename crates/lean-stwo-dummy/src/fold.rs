//! Same-statement fold (RESEARCH-PACK Phase 1B / rec 2, JiangXb-son):
//! N instances of `c = 3a+5b+7` share ONE AIR / ONE FRI proof / ONE verify.
//! Dummy DSTW × N is not an aggregate.

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
    dummy_m31_hash, CIRCUIT_TYPE_STWO, CURVE_TYPE_M31, M31, MAX_PROOF_BYTES, STWO_MAGIC,
    VerifyError,
};

/// Wire kind: same-statement fold (not a single ab-hash, not Dummy).
pub const FOLD_KIND: &[u8; 4] = b"FOLD";

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

fn log_n_rows(n: usize) -> u32 {
    let need = n.next_power_of_two().max(1 << LOG_N_LANES) as u32;
    need.trailing_zeros()
}

fn log_sizes(log_n: u32) -> stwo::core::pcs::TreeVec<Vec<u32>> {
    let info = HashEval { log_n_rows: log_n }.evaluate(InfoEvaluator::empty());
    let mut sizes = info.mask_offsets.as_cols_ref().map_cols(|_| log_n);
    sizes[PREPROCESSED_TRACE_IDX] = vec![];
    sizes
}

fn gen_trace(
    instances: &[(M31, M31, M31)],
    log_n: u32,
) -> Vec<CircleEvaluation<SimdBackend, StwoM31, BitReversedOrder>> {
    let n = 1usize << log_n;
    let domain = CanonicCoset::new(log_n).circle_domain();
    let pad_a = StwoM31::from_u32_unchecked(0);
    let pad_b = StwoM31::from_u32_unchecked(0);
    let pad_c = StwoM31::from_u32_unchecked(dummy_m31_hash(M31(0), M31(0)).0);
    let mut col_a = vec![pad_a; n];
    let mut col_b = vec![pad_b; n];
    let mut col_c = vec![pad_c; n];
    for (i, (a, b, c)) in instances.iter().enumerate() {
        col_a[i] = StwoM31::from_u32_unchecked(a.0);
        col_b[i] = StwoM31::from_u32_unchecked(b.0);
        col_c[i] = StwoM31::from_u32_unchecked(c.0);
    }
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

fn mix_public(channel: &mut Blake2sChannel, instances: &[(M31, M31, M31)]) {
    channel.mix_u64(instances.len() as u64);
    for (a, b, c) in instances {
        channel.mix_u64(a.0 as u64);
        channel.mix_u64(b.0 as u64);
        channel.mix_u64(c.0 as u64);
    }
}

/// Header: STWO | prover_id | curve_id | FOLD | n_le | (a,b,c)*n
fn header(instances: &[(M31, M31, M31)]) -> Vec<u8> {
    let mut out = Vec::new();
    out.extend_from_slice(STWO_MAGIC);
    out.push(CIRCUIT_TYPE_STWO);
    out.push(CURVE_TYPE_M31);
    out.extend_from_slice(FOLD_KIND);
    out.extend_from_slice(&(instances.len() as u32).to_le_bytes());
    for (a, b, c) in instances {
        out.extend_from_slice(&a.to_le_bytes());
        out.extend_from_slice(&b.to_le_bytes());
        out.extend_from_slice(&c.to_le_bytes());
    }
    out
}

fn parse_header(proof: &[u8]) -> Result<(Vec<(M31, M31, M31)>, usize), VerifyError> {
    if proof.len() > MAX_PROOF_BYTES {
        return Err(VerifyError::ProofTooLarge { len: proof.len() });
    }
    if proof.len() < 4 + 1 + 1 + 4 + 4 {
        return Err(VerifyError::Truncated);
    }
    if &proof[0..4] != STWO_MAGIC {
        return Err(VerifyError::BadMagic);
    }
    // Dummy-N: DSTW never reaches here; also reject DSTW after first chunk.
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
    let n = u32::from_le_bytes(proof[10..14].try_into().unwrap()) as usize;
    if n == 0 || n > 1 << 16 {
        return Err(VerifyError::Truncated);
    }
    let hdr = 14 + n * 12;
    if proof.len() < hdr {
        return Err(VerifyError::Truncated);
    }
    let mut inst = Vec::with_capacity(n);
    let mut off = 14;
    for _ in 0..n {
        let a = M31::from_le_bytes(proof[off..off + 4].try_into().unwrap())?;
        let b = M31::from_le_bytes(proof[off + 4..off + 8].try_into().unwrap())?;
        let c = M31::from_le_bytes(proof[off + 8..off + 12].try_into().unwrap())?;
        if dummy_m31_hash(a, b) != c {
            return Err(VerifyError::StatementFalse);
        }
        inst.push((a, b, c));
        off += 12;
    }
    Ok((inst, hdr))
}

pub fn prove_fold(pairs: &[(M31, M31)]) -> Result<Vec<u8>, VerifyError> {
    if pairs.is_empty() {
        return Err(VerifyError::Truncated);
    }
    let instances: Vec<(M31, M31, M31)> = pairs
        .iter()
        .map(|(a, b)| (*a, *b, dummy_m31_hash(*a, *b)))
        .collect();
    let log_n = log_n_rows(instances.len());
    let config = PcsConfig::default();
    let twiddles = SimdBackend::precompute_twiddles(
        CanonicCoset::new(log_n + config.fri_config.log_blowup_factor + 1)
            .circle_domain()
            .half_coset,
    );
    let channel = &mut Blake2sChannel::default();
    config.mix_into(channel);
    let mut commitment_scheme =
        CommitmentSchemeProver::<_, Blake2sMerkleChannel>::new(config, &twiddles);

    let tree_builder = commitment_scheme.tree_builder();
    tree_builder.commit(channel);

    mix_public(channel, &instances);

    let mut tree_builder = commitment_scheme.tree_builder();
    tree_builder.extend_evals(gen_trace(&instances, log_n));
    tree_builder.commit(channel);

    let component = HashComponent::new(
        &mut TraceLocationAllocator::default(),
        HashEval { log_n_rows: log_n },
        QM31::zero(),
    );

    let stark_proof =
        prove(&[&component], channel, commitment_scheme).map_err(|_| VerifyError::StwoVerify)?;
    let body = bincode::serialize(&stark_proof).map_err(|_| VerifyError::StwoVerify)?;
    let mut out = header(&instances);
    out.extend_from_slice(&body);
    if out.len() > MAX_PROOF_BYTES {
        return Err(VerifyError::ProofTooLarge { len: out.len() });
    }
    Ok(out)
}

pub fn verify_fold(proof: &[u8], pairs: &[(M31, M31)]) -> Result<(), VerifyError> {
    let (instances, hdr) = parse_header(proof)?;
    if instances.len() != pairs.len() {
        return Err(VerifyError::PublicInputMismatch);
    }
    for (got, want) in instances.iter().zip(pairs.iter()) {
        if got.0 != want.0 || got.1 != want.1 {
            return Err(VerifyError::PublicInputMismatch);
        }
    }
    let log_n = log_n_rows(instances.len());
    let stark_proof: StarkProof<Blake2sMerkleHasher> =
        bincode::deserialize(&proof[hdr..]).map_err(|_| VerifyError::StwoVerify)?;

    let pcs_config = stark_proof.config;
    let channel = &mut Blake2sChannel::default();
    pcs_config.mix_into(channel);
    let commitment_scheme = &mut CommitmentSchemeVerifier::<Blake2sMerkleChannel>::new(pcs_config);
    let sizes = log_sizes(log_n);
    commitment_scheme.commit(stark_proof.commitments[0], &sizes[0], channel);
    mix_public(channel, &instances);
    commitment_scheme.commit(stark_proof.commitments[1], &sizes[1], channel);

    let component = HashComponent::new(
        &mut TraceLocationAllocator::default(),
        HashEval { log_n_rows: log_n },
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
