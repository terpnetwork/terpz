//! TDD: RESEARCH-PACK-CONSENSUS-OBJECT.md § Phase 1B + rec #2 (`loop_id=aggregate`).
//!
//! Cite pack § Phase 1B: "JiangXb-son: same statement → compose the polynomial
//! before FRI. Terp: Lean aggregation is a named host (Stwo prover_id=2 / M31),
//! never CircuitType::Stark, never Dummy 3a+5b+7."
//! Cite pack rec #2: "N proofs → one real verify. Fail if N Dummy checksums."
//! Cite pack § What we must NOT: SHA256-RLC, FRIC counters, BlockedStwoHost as green.
//!
//! These tests **fail** Dummy DSTW, SHA256-of-PIs, FRIC, prover_id=2 stamps without
//! a FRI query/fold, and `AggError::BlockedStwoHost` standing in for a verify.

use crate::*;

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

fn is_sha256_fric_costume(blob: &AggregateBlob, enc: &[u8]) -> bool {
    enc.starts_with(b"FRIC") && blob.prover_id == CIRCUIT_TYPE_STWO
}

#[test]
fn dummy_dstw_never_aggregates() {
    let dummy = b"DSTW\x02\x05".to_vec();
    assert_eq!(
        reject_dummy_batch(&[dummy.clone(), dummy]),
        Err(AggError::DummyChecksum)
    );
    assert_eq!(
        decode_blob(b"DSTW\x02\x05abcdefgh"),
        Err(AggError::DummyChecksum)
    );
}

#[test]
fn fric_sha256_of_pis_is_not_stwo_fri_verify() {
    let leaves = vec![leaf(7, 0b01, 9), leaf(7, 0b10, 9), leaf(7, 0b100, 9)];
    let blob = compose_same_statement(&leaves).expect("compose N>=2 same-statement leaves");
    let enc = encode_blob(&blob).expect("encode");
    assert!(!enc.starts_with(b"DSTW"), "DSTW is not an aggregate");
    if is_sha256_fric_costume(&blob, &enc) {
        panic!(
            "FRIC + SHA256-of-evals + prover_id=2 stamp is not Stwo/FRI verify \
             (pack § Phase 1B: compose the polynomial before FRI; § What we must NOT)"
        );
    }
    if verify_aggregate(&blob, &leaves).is_ok() && enc.starts_with(b"FRIC") {
        panic!("verify_aggregate Ok on FRIC counters — not a real FRI query/fold");
    }
}

#[test]
fn prover_id_2_stamp_without_fri_fold_fails() {
    let leaves = vec![leaf(3, 1, 4), leaf(3, 2, 4)];
    let blob = compose_same_statement(&leaves).expect("compose");
    let enc = encode_blob(&blob).expect("encode");
    if blob.prover_id == CIRCUIT_TYPE_STWO && enc.starts_with(b"FRIC") {
        panic!(
            "prover_id=2 + FRIC/SHA256-of-PIs stamp is not Stwo/FRI \
             (pack § Phase 1B named host, never Dummy/FRIC costume)"
        );
    }
    verify_aggregate(&blob, &leaves).expect("named Stwo verify, not a stamp");
}

#[test]
fn n_leaves_must_be_one_real_verify_not_n_checksums() {
    let leaves = vec![leaf(1, 1, 1), leaf(1, 2, 1), leaf(1, 4, 1), leaf(1, 8, 1)];
    let blob = match compose_same_statement(&leaves) {
        Ok(b) => b,
        Err(AggError::BlockedStwoHost) => {
            panic!(
                "BlockedStwoHost is not N→1 Stwo/FRI fold (pack § Phase 1B + rec #2; \
                 pack § What we must NOT: BlockedStwoHost stubs as green)"
            )
        }
        Err(e) => panic!("N period/vote proofs must compose to one blob: {e:?}"),
    };
    assert_eq!(blob.n_leaves as usize, 4, "folded leaf count");
    assert_eq!(blob.prover_id, CIRCUIT_TYPE_STWO, "named Stwo prover_id=2");
    assert_eq!(blob.curve_id, CURVE_TYPE_M31, "M31 / curve_id=5");
    let enc = encode_blob(&blob).expect("encode composed blob");
    assert!(enc.len() >= 4, "aggregate body");
    assert_ne!(&enc[0..4], b"DSTW", "DSTW Dummy is not the fold");
    assert_ne!(&enc[0..4], b"FRIC", "FRIC SHA256-RLC is not compose-before-FRI");
    assert!(
        !is_sha256_fric_costume(&blob, &enc),
        "one verify must be Stwo/FRI fold, not FRIC SHA256-of-PIs"
    );
    match verify_aggregate(&blob, &leaves) {
        Ok(()) => {}
        Err(AggError::BlockedStwoHost) => {
            panic!("verify BlockedStwoHost is not one in-process Stwo/FRI verify")
        }
        Err(e) => panic!("one real Stwo/FRI verify: {e:?}"),
    }
}

#[test]
fn blocked_host_is_not_the_aggregate_property() {
    let leaves = vec![leaf(9, 0x0f, 0xaa), leaf(9, 0xf0, 0xaa)];
    match compose_same_statement(&leaves) {
        Err(AggError::BlockedStwoHost) => {
            panic!("loop_id=aggregate is not done until named Stwo fold verifies (not Blocked)")
        }
        Ok(blob) => {
            verify_aggregate(&blob, &leaves)
                .expect("compose without verify is not one FRI check");
        }
        Err(e) => panic!("unexpected compose error: {e:?}"),
    }
}
