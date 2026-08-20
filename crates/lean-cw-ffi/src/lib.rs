//! Commonware simplex C ABI for Lean AppState.
//!
//! Automaton.propose/verify map to Go PrepareProposal/ProcessProposal.
//! Reporter/finalization map to Go FinalizeBlock/Commit.
//! Dummy DSTW still fails in Go Process. JOIN/LEAV stay LNPR.

#![allow(non_camel_case_types)]

pub mod automaton;
pub mod callbacks;
pub mod engine;
pub mod ffi;
pub mod payload;

pub use payload::Payload;

pub const LEAN_CW_OK: i32 = 0;
pub const LEAN_CW_ERR: i32 = -1;

/// C layout matching include/lean_cw.h `lean_cw_cfg`.
#[repr(C)]
pub struct lean_cw_cfg {
    pub private_key: [u8; 32],
    pub listen: *const std::os::raw::c_char,
    pub bootstrappers: *const std::os::raw::c_char,
    pub storage_dir: *const std::os::raw::c_char,
    pub namespace: *const std::os::raw::c_char,
    pub participants: *const u8,
    pub participants_len: usize,
    pub epoch: u64,
}

unsafe impl Send for lean_cw_cfg {}
unsafe impl Sync for lean_cw_cfg {}
