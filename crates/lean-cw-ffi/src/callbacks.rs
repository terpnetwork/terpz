use std::os::raw::c_void;

pub const DIGEST_LEN: usize = 32;

pub type ProposeFn = unsafe extern "C" fn(
    user: *mut c_void,
    epoch: u64,
    view: u64,
    parent: *const u8,
    digest: *mut u8,
    payload: *mut *mut u8,
    payload_len: *mut usize,
) -> i32;

pub type VerifyFn = unsafe extern "C" fn(
    user: *mut c_void,
    epoch: u64,
    view: u64,
    digest: *const u8,
    payload: *const u8,
    payload_len: usize,
) -> i32;

pub type CertifyFn =
    unsafe extern "C" fn(user: *mut c_void, epoch: u64, view: u64, digest: *const u8) -> i32;

pub type ReportFn =
    unsafe extern "C" fn(user: *mut c_void, kind: u32, epoch: u64, view: u64, digest: *const u8);

pub type FinalizeFn = unsafe extern "C" fn(
    user: *mut c_void,
    epoch: u64,
    view: u64,
    digest: *const u8,
    payload: *const u8,
    payload_len: usize,
    certificate: *const u8,
    certificate_len: usize,
);

pub type ParticipantsFn = unsafe extern "C" fn(
    user: *mut c_void,
    epoch: u64,
    pk_out: *mut *mut u8,
    pk_len: *mut usize,
) -> i32;

#[repr(C)]
#[derive(Clone, Copy)]
pub struct RawCallbacks {
    pub propose: Option<ProposeFn>,
    pub verify: Option<VerifyFn>,
    pub certify: Option<CertifyFn>,
    pub report: Option<ReportFn>,
    pub finalize: Option<FinalizeFn>,
    pub participants: Option<ParticipantsFn>,
    pub user: *mut c_void,
}

unsafe impl Send for RawCallbacks {}
unsafe impl Sync for RawCallbacks {}

impl RawCallbacks {
    pub fn propose(
        &self,
        epoch: u64,
        view: u64,
        parent: &[u8; DIGEST_LEN],
    ) -> Option<([u8; DIGEST_LEN], Vec<u8>)> {
        let f = self.propose?;
        let mut digest = [0u8; DIGEST_LEN];
        let mut payload: *mut u8 = std::ptr::null_mut();
        let mut payload_len: usize = 0;
        let rc = unsafe {
            f(
                self.user,
                epoch,
                view,
                parent.as_ptr(),
                digest.as_mut_ptr(),
                &mut payload,
                &mut payload_len,
            )
        };
        if rc != 0 {
            return None;
        }
        let bytes = copy_and_free(payload, payload_len);
        Some((digest, bytes))
    }

    pub fn verify(&self, epoch: u64, view: u64, digest: &[u8; DIGEST_LEN], payload: &[u8]) -> bool {
        let Some(f) = self.verify else {
            return false;
        };
        let rc = unsafe {
            f(
                self.user,
                epoch,
                view,
                digest.as_ptr(),
                payload.as_ptr(),
                payload.len(),
            )
        };
        rc == 1
    }

    pub fn certify(&self, epoch: u64, view: u64, digest: &[u8; DIGEST_LEN]) -> bool {
        let Some(f) = self.certify else {
            return true;
        };
        unsafe { f(self.user, epoch, view, digest.as_ptr()) == 1 }
    }

    pub fn report(&self, kind: u32, epoch: u64, view: u64, digest: &[u8; DIGEST_LEN]) {
        if let Some(f) = self.report {
            unsafe { f(self.user, kind, epoch, view, digest.as_ptr()) }
        }
    }

    pub fn finalize(
        &self,
        epoch: u64,
        view: u64,
        digest: &[u8; DIGEST_LEN],
        payload: &[u8],
        certificate: &[u8],
    ) {
        if let Some(f) = self.finalize {
            unsafe {
                f(
                    self.user,
                    epoch,
                    view,
                    digest.as_ptr(),
                    payload.as_ptr(),
                    payload.len(),
                    certificate.as_ptr(),
                    certificate.len(),
                )
            }
        }
    }

    /// BondedSet bits=1 pubkeys (concatenated 32-byte ed25519). None if unset.
    pub fn participants(&self, epoch: u64) -> Option<Vec<u8>> {
        let f = self.participants?;
        let mut pk_out: *mut u8 = std::ptr::null_mut();
        let mut pk_len: usize = 0;
        let rc = unsafe { f(self.user, epoch, &mut pk_out, &mut pk_len) };
        if rc != 0 {
            return None;
        }
        Some(copy_and_free(pk_out, pk_len))
    }
}

fn copy_and_free(ptr: *mut u8, len: usize) -> Vec<u8> {
    if ptr.is_null() || len == 0 {
        return Vec::new();
    }
    let bytes = unsafe { std::slice::from_raw_parts(ptr, len).to_vec() };
    unsafe { libc::free(ptr as *mut libc::c_void) };
    bytes
}
