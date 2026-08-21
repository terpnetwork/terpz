//! Lean certificate wire: pubkey index + real ed25519 sig bytes.
//!
//! Not `AA==`. Not committee paint. Used by `/block` last_commit and `/lean/certificate`.

use crate::scheme::WeightedScheme;
use commonware_codec::Encode;
use commonware_consensus::{simplex::types::Finalization, Epochable, Viewable};
use commonware_cryptography::sha256::Digest as ShaDigest;
use commonware_utils::Participant;

pub const MAGIC: &[u8; 5] = b"LCERT";
pub const VERSION: u8 = 1;

/// Encode attributable Finalization signers for Go RPC / catch-up.
pub fn encode_lcert(f: &Finalization<WeightedScheme, ShaDigest>) -> Vec<u8> {
    let raw = f.encode();
    let epoch = f.epoch().get();
    let view = f.view().get();
    let digest = f.proposal.payload.as_ref();
    let cert = &f.certificate;
    let n = cert.signers.count() as u32;

    let mut out =
        Vec::with_capacity(5 + 1 + 8 + 8 + 32 + 4 + (n as usize) * (4 + 64) + 4 + raw.len());
    out.extend_from_slice(MAGIC);
    out.push(VERSION);
    out.extend_from_slice(&epoch.to_be_bytes());
    out.extend_from_slice(&view.to_be_bytes());
    out.extend_from_slice(digest);
    out.extend_from_slice(&n.to_be_bytes());
    let mut sigs = cert.signatures.iter();
    for signer in cert.signers.iter() {
        let idx: u32 = signer.get();
        out.extend_from_slice(&idx.to_be_bytes());
        match sigs.next().and_then(|s| s.get()) {
            Some(sig) => {
                let b = sig.as_ref();
                let mut slot = [0u8; 64];
                let n = b.len().min(64);
                slot[..n].copy_from_slice(&b[..n]);
                out.extend_from_slice(&slot);
            }
            None => out.extend_from_slice(&[0u8; 64]),
        }
    }
    let raw_len = raw.len() as u32;
    out.extend_from_slice(&raw_len.to_be_bytes());
    out.extend_from_slice(&raw);
    out
}

pub fn contains_dstw(payload: &[u8]) -> bool {
    payload.windows(4).any(|w| w == b"DSTW")
}

#[allow(dead_code)]
pub fn signer_index(p: Participant) -> u32 {
    p.get()
}
