//! Opaque simplex payload: Go Prepare/Process txs with a height header.
//!
//! Wire: `LNCW` | version(1) | height(u64 BE) | n(u32 BE) | (len(u32 BE)+bytes)*n

pub const MAGIC: &[u8; 4] = b"LNCW";
pub const VERSION: u8 = 1;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Payload {
    pub height: u64,
    pub txs: Vec<Vec<u8>>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PayloadError {
    Truncated,
    BadMagic,
    BadVersion,
}

impl Payload {
    pub fn encode(&self) -> Vec<u8> {
        let mut out = Vec::new();
        out.extend_from_slice(MAGIC);
        out.push(VERSION);
        out.extend_from_slice(&self.height.to_be_bytes());
        out.extend_from_slice(&(self.txs.len() as u32).to_be_bytes());
        for tx in &self.txs {
            out.extend_from_slice(&(tx.len() as u32).to_be_bytes());
            out.extend_from_slice(tx);
        }
        out
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, PayloadError> {
        if bytes.len() < 4 + 1 + 8 + 4 {
            return Err(PayloadError::Truncated);
        }
        if &bytes[0..4] != MAGIC {
            return Err(PayloadError::BadMagic);
        }
        if bytes[4] != VERSION {
            return Err(PayloadError::BadVersion);
        }
        let height = u64::from_be_bytes(bytes[5..13].try_into().unwrap());
        let n = u32::from_be_bytes(bytes[13..17].try_into().unwrap()) as usize;
        let mut i = 17;
        let mut txs = Vec::with_capacity(n);
        for _ in 0..n {
            if i + 4 > bytes.len() {
                return Err(PayloadError::Truncated);
            }
            let len = u32::from_be_bytes(bytes[i..i + 4].try_into().unwrap()) as usize;
            i += 4;
            if i + len > bytes.len() {
                return Err(PayloadError::Truncated);
            }
            txs.push(bytes[i..i + len].to_vec());
            i += len;
        }
        Ok(Self { height, txs })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn roundtrip_empty() {
        let p = Payload {
            height: 1,
            txs: vec![],
        };
        assert_eq!(Payload::decode(&p.encode()).unwrap(), p);
    }

    #[test]
    fn roundtrip_lnpr_and_join() {
        let p = Payload {
            height: 7,
            txs: vec![b"LNPR....".to_vec(), b"JOIN....".to_vec()],
        };
        let got = Payload::decode(&p.encode()).unwrap();
        assert_eq!(got, p);
    }

    #[test]
    fn dummy_dstw_bytes_pass_through() {
        // Dummy DSTW is application-verified in Go Process, not here.
        let mut proof = b"DSTW".to_vec();
        proof.extend_from_slice(&[2u8, 5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
        let p = Payload {
            height: 2,
            txs: vec![proof],
        };
        let enc = p.encode();
        let got = Payload::decode(&enc).unwrap();
        assert_eq!(got.txs[0][0..4], *b"DSTW");
        assert_eq!(got.txs[0][4], 2);
        assert_eq!(got.txs[0][5], 5);
    }

    #[test]
    fn reject_truncated() {
        assert_eq!(Payload::decode(b"LNC"), Err(PayloadError::Truncated));
        assert_eq!(Payload::decode(b"XXXX............."), Err(PayloadError::BadMagic));
    }
}
