//! Weighted ed25519 scheme adapter.
//!
//! Simplex `N3f1` is one vote per pubkey. BondedSet EB is the power SoT.
//! Quorum for Lean is strict >2/3 of bonded weight.
//!
//! Commonware's batcher calls `assemble` only after `N3f1::quorum(n)` verified
//! votes and then `expect`s success. Returning `None` there panics the engine.
//! So `assemble` still delegates to ed25519 `N3f1`. Weighted stall is enforced
//! in `verify_certificate` and in the Go Commit path: a certificate whose
//! signers hold silent complementary weight >= 1/3 is rejected, so height stops.

use commonware_codec::{Decode, Encode};
use commonware_consensus::simplex::{
    scheme::ed25519::Scheme as Inner,
    types::{Finalization, Subject},
};
use commonware_cryptography::{
    certificate::{Attestation, Scheme as CertScheme, Verification, Verifier as CertVerifier},
    ed25519::{certificate::Certificate, PrivateKey, PublicKey},
    Digest,
};
use commonware_parallel::Strategy;
use commonware_utils::{ordered::Set, N3f1, Participant};
use rand_core::CryptoRng;
use std::fmt::Debug;

/// Strict >2/3 of `total` bonded weight.
pub fn meets_weight(signed: u64, total: u64) -> bool {
    total > 0 && signed.saturating_mul(3) > total.saturating_mul(2)
}

#[derive(Clone, Debug)]
pub struct WeightedScheme {
    inner: Inner,
    weights: Vec<u64>,
    total: u64,
}

impl WeightedScheme {
    pub fn signer(
        namespace: &[u8],
        participants: Set<PublicKey>,
        private_key: PrivateKey,
        weights: Vec<u64>,
    ) -> Result<Self, String> {
        let inner = Inner::signer(namespace, participants, private_key)
            .ok_or_else(|| "private key is not in participants".to_string())?;
        Self::wrap(inner, weights)
    }

    pub fn verifier(
        namespace: &[u8],
        participants: Set<PublicKey>,
        weights: Vec<u64>,
    ) -> Result<Self, String> {
        Self::wrap(Inner::verifier(namespace, participants), weights)
    }

    fn wrap(inner: Inner, mut weights: Vec<u64>) -> Result<Self, String> {
        let n = inner.participants().len();
        if weights.is_empty() {
            weights = vec![1; n];
        }
        if weights.len() != n {
            return Err(format!(
                "weights len {} != participants {n} (do not repeat pubkeys)",
                weights.len()
            ));
        }
        let total: u64 = weights.iter().copied().sum();
        if total == 0 {
            return Err("BondedSet total weight is 0".into());
        }
        Ok(Self {
            inner,
            weights,
            total,
        })
    }

    pub fn weight_of(&self, p: Participant) -> u64 {
        self.weights.get(p.get() as usize).copied().unwrap_or(0)
    }

    pub fn total_weight(&self) -> u64 {
        self.total
    }

    pub fn signed_weight(&self, signers: impl Iterator<Item = Participant>) -> u64 {
        signers.map(|p| self.weight_of(p)).sum()
    }

    pub fn certificate_meets_weight(&self, cert: &Certificate) -> bool {
        meets_weight(self.signed_weight(cert.signers.iter()), self.total)
    }
}

impl CertVerifier for WeightedScheme {
    type Subject<'a, D: Digest> = Subject<'a, D>;
    type Faults = N3f1;
    type PublicKey = PublicKey;
    type Certificate = Certificate;

    fn verify_certificate<R, D>(
        &self,
        rng: &mut R,
        subject: Self::Subject<'_, D>,
        certificate: &Self::Certificate,
        strategy: &impl Strategy,
    ) -> bool
    where
        R: CryptoRng,
        D: Digest,
    {
        if !self.certificate_meets_weight(certificate) {
            tracing::warn!(
                signed = self.signed_weight(certificate.signers.iter()),
                total = self.total,
                "certificate below 2/3 BondedSet weight"
            );
            return false;
        }
        self.inner
            .verify_certificate(rng, subject, certificate, strategy)
    }

    fn is_batchable() -> bool {
        Inner::is_batchable()
    }

    fn certificate_codec_config(&self) -> <Self::Certificate as commonware_codec::Read>::Cfg {
        self.inner.certificate_codec_config()
    }

    fn certificate_codec_config_unbounded() -> <Self::Certificate as commonware_codec::Read>::Cfg {
        Inner::certificate_codec_config_unbounded()
    }
}

impl CertScheme for WeightedScheme {
    type Signature = commonware_cryptography::ed25519::Signature;

    fn me(&self) -> Option<Participant> {
        self.inner.me()
    }

    fn participants(&self) -> &Set<Self::PublicKey> {
        self.inner.participants()
    }

    fn sign<D: Digest>(&self, subject: Self::Subject<'_, D>) -> Option<Attestation<Self>> {
        self.inner.sign(subject).map(|a| Attestation {
            signer: a.signer,
            signature: a.signature,
        })
    }

    fn verify_attestation<R, D>(
        &self,
        rng: &mut R,
        subject: Self::Subject<'_, D>,
        attestation: &Attestation<Self>,
        strategy: &impl Strategy,
    ) -> bool
    where
        R: CryptoRng,
        D: Digest,
    {
        let inner_att = Attestation::<Inner> {
            signer: attestation.signer,
            signature: attestation.signature.clone(),
        };
        self.inner
            .verify_attestation(rng, subject, &inner_att, strategy)
    }

    fn verify_attestations<R, D, I>(
        &self,
        rng: &mut R,
        subject: Self::Subject<'_, D>,
        attestations: I,
        strategy: &impl Strategy,
    ) -> Verification<Self>
    where
        R: CryptoRng,
        D: Digest,
        I: IntoIterator<Item = Attestation<Self>>,
        I::IntoIter: Send,
    {
        let mapped: Vec<Attestation<Inner>> = attestations
            .into_iter()
            .map(|a| Attestation {
                signer: a.signer,
                signature: a.signature,
            })
            .collect();
        let inner = self
            .inner
            .verify_attestations(rng, subject, mapped, strategy);
        Verification::new(
            inner
                .verified
                .into_iter()
                .map(|a| Attestation {
                    signer: a.signer,
                    signature: a.signature,
                })
                .collect(),
            inner.invalid,
        )
    }

    fn assemble<I>(&self, attestations: I, strategy: &impl Strategy) -> Option<Self::Certificate>
    where
        I: IntoIterator<Item = Attestation<Self>>,
        I::IntoIter: Send,
    {
        // Must succeed whenever the batcher has N3f1 verified votes (it `expect`s).
        // Weight is enforced in verify_certificate + Go Commit.
        let mapped: Vec<Attestation<Inner>> = attestations
            .into_iter()
            .map(|a| Attestation {
                signer: a.signer,
                signature: a.signature,
            })
            .collect();
        self.inner.assemble(mapped, strategy)
    }

    fn is_attributable() -> bool {
        Inner::is_attributable()
    }
}

/// Decode a persisted Finalization against this scheme's participant count.
pub fn decode_finalization(
    scheme: &WeightedScheme,
    bytes: &[u8],
) -> Option<Finalization<WeightedScheme, commonware_cryptography::sha256::Digest>> {
    let n = scheme.participants().len();
    Finalization::<WeightedScheme, commonware_cryptography::sha256::Digest>::decode_cfg(bytes, &n)
        .ok()
}

pub fn encode_finalization(
    f: &Finalization<WeightedScheme, commonware_cryptography::sha256::Digest>,
) -> Vec<u8> {
    f.encode().to_vec()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn weight_quorum_equal_matches_n3f1_4() {
        assert!(meets_weight(30, 40));
        assert!(!meets_weight(20, 40));
    }

    #[test]
    fn silent_third_stalls() {
        assert!(!meets_weight(3, 103));
        assert!(meets_weight(101, 103));
    }
}
