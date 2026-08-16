//! Phase 1A valset AIR (LEAN-3): 5-byte deposit index + 1-byte EB.
//! Fail-closed stub — no circuit. Real Stwo prove/verify is the implementer's job.

use crate::VerifyError;

/// Beacon-style deposit-tree index (not a 32-byte pubkey).
pub const VALSET_INDEX_LEN: usize = 5;
/// On-chain effective balance (1 byte).
pub const VALSET_EB_LEN: usize = 1;
/// ~6-byte Phase 1 validator row.
pub const VALSET_STATE_LEN: usize = VALSET_INDEX_LEN + VALSET_EB_LEN;

/// Public instance: period (8 BE) || deposit_index (5) || eb (1).
pub fn valset_instance_bytes(period: u64, deposit_index: [u8; 5], eb: u8) -> Vec<u8> {
    let mut out = Vec::with_capacity(8 + VALSET_STATE_LEN);
    out.extend_from_slice(&period.to_be_bytes());
    out.extend_from_slice(&deposit_index);
    out.push(eb);
    out
}

/// Real Stwo prove. Not DummyStwo / DSTW.
pub fn prove_valset(
    _period: u64,
    _deposit_index: [u8; 5],
    _eb: u8,
) -> Result<Vec<u8>, VerifyError> {
    Err(VerifyError::StwoVerify)
}

pub fn verify_valset(
    _proof: &[u8],
    _period: u64,
    _deposit_index: [u8; 5],
    _eb: u8,
) -> Result<(), VerifyError> {
    Err(VerifyError::StwoVerify)
}
