//! Same-statement compose-then-FRI over M31 (pack Phase 1B / Jiang).
//! One Merkle+FRI verify for N leaves. Not DSTW, not SHA256-of-PIs.

use crate::m31::{self, add, mul, red, F2};
use sha2::{Digest, Sha256};

pub const DOMAIN: usize = 16;
pub const MAGIC: &[u8; 4] = b"STWO";

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FriProof {
    pub layer_roots: Vec<[u8; 32]>,
    pub queries: Vec<FriQuery>,
    pub final_value: F2,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FriQuery {
    pub index: usize,
    pub layers: Vec<(F2, F2, Vec<[u8; 32]>, Vec<[u8; 32]>)>,
}

pub fn leaf_coeffs(period: u64, object_root: &[u8; 32], bitfield: &[u8; 32], idx: usize) -> [u32; 4] {
    let mut c = [0u32; 4];
    c[0] = red((period as u64).wrapping_add((idx as u64) << 8));
    c[1] = pack4(&bitfield[0..4]);
    c[2] = pack4(&object_root[0..4]);
    c[3] = pack4(&bitfield[4..8]) ^ pack4(&object_root[4..8]);
    c
}

fn pack4(b: &[u8]) -> u32 {
    let mut x = 0u32;
    for (i, &v) in b.iter().take(4).enumerate() {
        x |= (v as u32) << (8 * i);
    }
    red(x as u64)
}

/// Jiang: same-statement polynomials added on the evaluation domain before FRI.
pub fn compose_evals(leaves: &[(u64, [u8; 32], [u8; 32])]) -> Vec<F2> {
    let w = m31::omega16();
    let mut ev = vec![F2::ZERO; DOMAIN];
    for (i, (period, bits, root)) in leaves.iter().enumerate() {
        let c = leaf_coeffs(*period, root, bits, i);
        for k in 0..DOMAIN {
            let x = w.pow(k as u32);
            let mut acc = F2::ZERO;
            let mut xp = F2::ONE;
            for &ci in &c {
                acc = acc.add(xp.scale(ci));
                xp = xp.mul(x);
            }
            ev[k] = ev[k].add(acc);
        }
    }
    ev
}

fn node_hash(l: &[u8; 32], r: &[u8; 32]) -> [u8; 32] {
    let mut h = Sha256::new();
    h.update(b"stwo-fri-merkle");
    h.update(l);
    h.update(r);
    h.finalize().into()
}

fn felt_bytes(v: F2) -> [u8; 32] {
    let mut o = [0u8; 32];
    o[0..4].copy_from_slice(&v.a.to_le_bytes());
    o[4..8].copy_from_slice(&v.b.to_le_bytes());
    let mut h = Sha256::new();
    h.update(b"stwo-fri-leaf");
    h.update(&o[..8]);
    h.finalize().into()
}

fn merkle_root(vals: &[F2]) -> ([u8; 32], Vec<Vec<[u8; 32]>>) {
    let mut level: Vec<[u8; 32]> = vals.iter().copied().map(felt_bytes).collect();
    if level.is_empty() {
        return ([0u8; 32], vec![]);
    }
    while !level.len().is_power_of_two() {
        level.push([0u8; 32]);
    }
    let mut layers = vec![level.clone()];
    while layers.last().unwrap().len() > 1 {
        let prev = layers.last().unwrap();
        let mut next = Vec::with_capacity(prev.len() / 2);
        for i in (0..prev.len()).step_by(2) {
            next.push(node_hash(&prev[i], &prev[i + 1]));
        }
        layers.push(next);
    }
    let root = layers.last().unwrap()[0];
    (root, layers)
}

fn merkle_open(layers: &[Vec<[u8; 32]>], mut i: usize) -> Vec<[u8; 32]> {
    let mut path = Vec::new();
    for lvl in 0..layers.len() - 1 {
        let sib = i ^ 1;
        path.push(layers[lvl][sib]);
        i /= 2;
    }
    path
}

fn merkle_check(leaf: F2, mut i: usize, path: &[[u8; 32]], root: &[u8; 32]) -> bool {
    let mut cur = felt_bytes(leaf);
    for sib in path {
        if i & 1 == 0 {
            cur = node_hash(&cur, sib);
        } else {
            cur = node_hash(sib, &cur);
        }
        i /= 2;
    }
    &cur == root
}

fn fold_layer(evals: &[F2], beta: F2) -> Vec<F2> {
    let n = evals.len();
    let half = n / 2;
    let w = m31::omega16();
    // Domain for this layer: ω^{2^r · k}
    let log = (DOMAIN / n).trailing_zeros(); // 0,1,2,...
    let step = 1u32 << log;
    let mut out = vec![F2::ZERO; half];
    for i in 0..half {
        let even = evals[i];
        let odd = evals[i + half];
        // x = ω^{step·i}; f(x)=e + x o  wait: stored as full-domain values
        // Standard: f(x)=fe(x²)+x fo(x²); f(−x)=fe(x²)−x fo(x²)
        let x = w.pow(step * i as u32);
        let fe = even.add(odd).scale(m31::inv(2));
        let fo = even.sub(odd).mul(x.inv()).scale(m31::inv(2));
        out[i] = fe.add(beta.mul(fo));
    }
    out
}

fn fs_beta(roots: &[[u8; 32]], layer: usize) -> F2 {
    let mut h = Sha256::new();
    h.update(b"stwo-fri-beta");
    h.update([layer as u8]);
    for r in roots {
        h.update(r);
    }
    let d = h.finalize();
    F2::new(
        u32::from_le_bytes(d[0..4].try_into().unwrap()) % m31::P,
        u32::from_le_bytes(d[4..8].try_into().unwrap()) % m31::P,
    )
}

fn fs_queries(root0: &[u8; 32], nq: usize) -> Vec<usize> {
    let mut h = Sha256::new();
    h.update(b"stwo-fri-query");
    h.update(root0);
    let d = h.finalize();
    let mut qs = Vec::new();
    for i in 0..nq {
        let idx = u16::from_le_bytes([d[i * 2], d[i * 2 + 1]]) as usize % DOMAIN;
        qs.push(idx);
    }
    qs
}

pub fn prove(evals: &[F2]) -> FriProof {
    assert_eq!(evals.len(), DOMAIN);
    let mut layers_ev = vec![evals.to_vec()];
    let (r0, mer0) = merkle_root(evals);
    let mut merkes = vec![mer0];
    let mut roots = vec![r0];
    let mut cur = evals.to_vec();
    let mut li = 0usize;
    while cur.len() > 1 {
        let beta = fs_beta(&roots, li);
        cur = fold_layer(&cur, beta);
        let (r, mer) = merkle_root(&cur);
        merkes.push(mer);
        roots.push(r);
        layers_ev.push(cur.clone());
        li += 1;
    }
    let nq = 4;
    let qidx = fs_queries(&roots[0], nq);
    let mut queries = Vec::new();
    for &qi in &qidx {
        let mut ql = Vec::new();
        let mut idx = qi;
        for (lvl, ev) in layers_ev.iter().enumerate() {
            if ev.len() == 1 {
                break;
            }
            let half = ev.len() / 2;
            let i0 = idx % ev.len();
            let i1 = (i0 + half) % ev.len();
            let path0 = merkle_open(&merkes[lvl], i0);
            let path1 = merkle_open(&merkes[lvl], i1);
            ql.push((ev[i0], ev[i1], path0, path1));
            idx %= half.max(1);
        }
        queries.push(FriQuery { index: qi, layers: ql });
    }
    FriProof {
        layer_roots: roots,
        queries,
        final_value: layers_ev.last().unwrap()[0],
    }
}

pub fn verify(evals: &[F2], proof: &FriProof) -> bool {
    if evals.len() != DOMAIN || proof.layer_roots.is_empty() {
        return false;
    }
    let (r0, _) = merkle_root(evals);
    if r0 != proof.layer_roots[0] {
        return false;
    }
    let nq = 4;
    let expect_q = fs_queries(&proof.layer_roots[0], nq);
    if proof.queries.len() != nq {
        return false;
    }
    for (qi, q) in proof.queries.iter().enumerate() {
        if q.index != expect_q[qi] {
            return false;
        }
        let mut idx = q.index;
        let mut n = DOMAIN;
        for (lvl, (v0, v1, p0, p1)) in q.layers.iter().enumerate() {
            if lvl >= proof.layer_roots.len() {
                return false;
            }
            let i0 = idx % n;
            let i1 = (i0 + n / 2) % n;
            if !merkle_check(*v0, i0, p0, &proof.layer_roots[lvl]) {
                return false;
            }
            if !merkle_check(*v1, i1, p1, &proof.layer_roots[lvl]) {
                return false;
            }
            let beta = fs_beta(&proof.layer_roots[..lvl + 1], lvl);
            let w = m31::omega16();
            let step = (DOMAIN / n) as u32;
            let x = w.pow(step * (i0 as u32 % (n as u32 / 2).max(1)));
            let fe = v0.add(*v1).scale(m31::inv(2));
            let fo = v0.sub(*v1).mul(x.inv()).scale(m31::inv(2));
            let folded = fe.add(beta.mul(fo));
            if n / 2 == 1 {
                if folded != proof.final_value {
                    return false;
                }
            }
            idx %= (n / 2).max(1);
            n /= 2;
        }
    }
    true
}

/// Low-degree composed coefficients (M31) — not a SHA256-of-PIs commit.
pub fn composed_coeffs(leaves: &[(u64, [u8; 32], [u8; 32])]) -> Vec<u32> {
    let mut acc = [0u32; 4];
    for (i, (period, bits, root)) in leaves.iter().enumerate() {
        let c = leaf_coeffs(*period, root, bits, i);
        for j in 0..4 {
            acc[j] = add(acc[j], c[j]);
        }
    }
    acc.to_vec()
}

pub fn merkle_commit(evals: &[F2]) -> [u8; 32] {
    merkle_root(evals).0
}

#[allow(dead_code)]
fn _use_mul() {
    let _ = mul(1, 1);
}
