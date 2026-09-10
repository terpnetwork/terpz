//! Mersenne-31 and quadratic extension used for two-adic FRI.
//! Circle STARKs use M31; p^2-1 is divisible by 2^31 so classic FRI lives in F_{p^2}.

pub const P: u32 = (1 << 31) - 1;

#[inline]
pub fn red(x: u64) -> u32 {
    let mut v = (x as u32 & P).wrapping_add((x >> 31) as u32);
    if v >= P {
        v -= P;
    }
    v
}

#[inline]
pub fn add(a: u32, b: u32) -> u32 {
    red(a as u64 + b as u64)
}

#[inline]
pub fn sub(a: u32, b: u32) -> u32 {
    red(a as u64 + P as u64 - b as u64)
}

#[inline]
pub fn mul(a: u32, b: u32) -> u32 {
    red(a as u64 * b as u64)
}

pub fn inv(a: u32) -> u32 {
    // a^{p-2} = a^{2^31-3}
    let mut b = a;
    let mut e = P - 2;
    let mut r = 1u32;
    while e > 0 {
        if e & 1 == 1 {
            r = mul(r, b);
        }
        b = mul(b, b);
        e >>= 1;
    }
    r
}

/// a + b·i with i² = −1 (non-residue in M31 since p ≡ 3 mod 4).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct F2 {
    pub a: u32,
    pub b: u32,
}

impl F2 {
    pub const ZERO: Self = Self { a: 0, b: 0 };
    pub const ONE: Self = Self { a: 1, b: 0 };

    pub fn new(a: u32, b: u32) -> Self {
        Self { a: a % P, b: b % P }
    }

    pub fn add(self, o: Self) -> Self {
        Self {
            a: add(self.a, o.a),
            b: add(self.b, o.b),
        }
    }

    pub fn sub(self, o: Self) -> Self {
        Self {
            a: sub(self.a, o.a),
            b: sub(self.b, o.b),
        }
    }

    pub fn mul(self, o: Self) -> Self {
        // (a+bi)(c+di) = (ac−bd) + (ad+bc)i
        Self {
            a: sub(mul(self.a, o.a), mul(self.b, o.b)),
            b: add(mul(self.a, o.b), mul(self.b, o.a)),
        }
    }

    pub fn scale(self, s: u32) -> Self {
        Self {
            a: mul(self.a, s),
            b: mul(self.b, s),
        }
    }

    pub fn inv(self) -> Self {
        // 1/(a+bi) = (a−bi)/(a²+b²)
        let n = add(mul(self.a, self.a), mul(self.b, self.b));
        let i = inv(n);
        Self {
            a: mul(self.a, i),
            b: mul(sub(0, self.b), i),
        }
    }

    pub fn pow(self, mut e: u32) -> Self {
        let mut b = self;
        let mut r = Self::ONE;
        while e > 0 {
            if e & 1 == 1 {
                r = r.mul(b);
            }
            b = b.mul(b);
            e >>= 1;
        }
        r
    }
}

/// Primitive 16-th root in F_{p^2}: ω^{16}=1, ω^8 = −1.
pub fn omega16() -> F2 {
    // g^{(p^2-1)/16} for g=2+i. exp = (P-1)*(P+1)/16 = (2^31-2)*2^27.
    let g = F2::new(2, 1);
    // Compute g^{2^27} first, then raise to (2^31-2).
    let mut h = g;
    for _ in 0..27 {
        h = h.mul(h);
    }
    // h = g^{2^27}; need h^{2^31-2} = h^{2(2^30-1)}
    h.pow(P - 1)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn omega16_order() {
        let w = omega16();
        let w8 = w.pow(8);
        assert_eq!(w8, F2::new(P - 1, 0)); // −1
        assert_eq!(w.pow(16), F2::ONE);
        assert_ne!(w.pow(8), F2::ONE);
    }
}
