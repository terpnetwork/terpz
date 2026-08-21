use crate::{
    callbacks::RawCallbacks,
    engine::{self, StartCfg, EPOCH, HEIGHT, RUNNING},
};
use commonware_codec::DecodeExt;
use std::{ffi::CStr, os::raw::c_char, slice, sync::atomic::Ordering, thread};

#[no_mangle]
pub extern "C" fn lean_cw_start(cfg: *const crate::lean_cw_cfg, cb: *const RawCallbacks) -> i32 {
    if cfg.is_null() || cb.is_null() {
        return crate::LEAN_CW_ERR;
    }
    let cfg = unsafe { &*cfg };
    let cb = unsafe { *cb };

    let mut private_key = [0u8; 32];
    private_key.copy_from_slice(&cfg.private_key);

    let listen = cstr(cfg.listen);
    let bootstrappers = cstr(cfg.bootstrappers);
    let storage_dir = cstr(cfg.storage_dir);
    let namespace = cstr(cfg.namespace);
    let participants = if cfg.participants.is_null() || cfg.participants_len == 0 {
        Vec::new()
    } else {
        match engine::parse_participants(unsafe {
            slice::from_raw_parts(cfg.participants, cfg.participants_len)
        }) {
            Ok(p) => p,
            Err(e) => {
                tracing::error!(%e, "lean_cw_start participants");
                return crate::LEAN_CW_ERR;
            }
        }
    };

    let weights = if cfg.weights.is_null() || cfg.weights_len == 0 {
        Vec::new()
    } else {
        unsafe { slice::from_raw_parts(cfg.weights, cfg.weights_len) }.to_vec()
    };
    let floor_path = cstr(cfg.floor_path);
    let floor_cert = if cfg.floor_cert.is_null() || cfg.floor_cert_len == 0 {
        Vec::new()
    } else {
        unsafe { slice::from_raw_parts(cfg.floor_cert, cfg.floor_cert_len) }.to_vec()
    };

    let start = StartCfg {
        private_key,
        listen,
        bootstrappers,
        storage_dir,
        namespace,
        participants,
        weights,
        epoch: cfg.epoch,
        floor_path,
        floor_cert,
    };

    let _ = tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::from_default_env()
                .add_directive("lean_cw_ffi=info".parse().unwrap()),
        )
        .try_init();

    thread::Builder::new()
        .name("lean-cw-ffi".into())
        .spawn(move || {
            if let Err(e) = engine::run_blocking(start, cb) {
                tracing::error!(%e, "lean-cw-ffi engine exited");
            }
        })
        .expect("spawn lean-cw-ffi thread");

    // Give the runner a moment to set RUNNING (fails fast if not in set).
    for _ in 0..100 {
        if RUNNING.load(Ordering::SeqCst) {
            return crate::LEAN_CW_OK;
        }
        thread::sleep(std::time::Duration::from_millis(20));
    }
    crate::LEAN_CW_ERR
}

#[no_mangle]
pub extern "C" fn lean_cw_stop() -> i32 {
    engine::request_stop();
    for _ in 0..200 {
        if !RUNNING.load(Ordering::SeqCst) {
            break;
        }
        thread::sleep(std::time::Duration::from_millis(20));
    }
    crate::LEAN_CW_OK
}

#[no_mangle]
pub extern "C" fn lean_cw_running() -> i32 {
    if RUNNING.load(Ordering::SeqCst) {
        1
    } else {
        0
    }
}

#[no_mangle]
pub extern "C" fn lean_cw_height() -> u64 {
    HEIGHT.load(Ordering::SeqCst)
}

#[no_mangle]
pub extern "C" fn lean_cw_epoch() -> u64 {
    EPOCH.load(Ordering::SeqCst)
}

#[no_mangle]
pub extern "C" fn lean_cw_free(p: *mut u8) {
    if !p.is_null() {
        unsafe { libc::free(p as *mut libc::c_void) }
    }
}

fn cstr(p: *const c_char) -> String {
    if p.is_null() {
        return String::new();
    }
    unsafe { CStr::from_ptr(p) }.to_string_lossy().into_owned()
}

/// Called from Go finalize to publish committed height (view != height).
#[no_mangle]
pub extern "C" fn lean_cw_set_height(height: u64) {
    HEIGHT.store(height, Ordering::SeqCst);
}

/// Copy last LCERT into malloc'd buffer. Returns length, 0 if none.
#[no_mangle]
pub extern "C" fn lean_cw_last_certificate(out: *mut *mut u8) -> usize {
    if out.is_null() {
        return 0;
    }
    let Some(bytes) = engine::last_lcert() else {
        unsafe { *out = std::ptr::null_mut() };
        return 0;
    };
    let len = bytes.len();
    let ptr = unsafe { libc::malloc(len) as *mut u8 };
    if ptr.is_null() {
        unsafe { *out = std::ptr::null_mut() };
        return 0;
    }
    unsafe {
        std::ptr::copy_nonoverlapping(bytes.as_ptr(), ptr, len);
        *out = ptr;
    }
    len
}

/// Verify a Commonware-encoded Finalization against participants+weights.
/// 1 = ok, 0 = reject.
#[no_mangle]
pub extern "C" fn lean_cw_verify_finalization(
    participants: *const u8,
    participants_len: usize,
    weights: *const u64,
    weights_len: usize,
    cert: *const u8,
    cert_len: usize,
) -> i32 {
    if participants.is_null() || cert.is_null() || cert_len == 0 {
        return 0;
    }
    let pbytes = unsafe { slice::from_raw_parts(participants, participants_len) };
    let Ok(pks) = engine::parse_participants(pbytes) else {
        return 0;
    };
    let w = if weights.is_null() || weights_len == 0 {
        vec![1u64; pks.len()]
    } else {
        unsafe { slice::from_raw_parts(weights, weights_len) }.to_vec()
    };
    let raw = unsafe { slice::from_raw_parts(cert, cert_len) };
    match verify_finalization_bytes(&pks, &w, raw) {
        true => 1,
        false => 0,
    }
}

fn verify_finalization_bytes(pks: &[[u8; 32]], weights: &[u64], raw: &[u8]) -> bool {
    use crate::scheme::WeightedScheme;
    use commonware_cryptography::ed25519;
    use commonware_parallel::Sequential;
    use commonware_utils::{ordered::Set, union, TryCollect};
    use rand::rngs::{StdRng, SysRng};
    use rand::{SeedableRng, TryRng};

    let mut keys = Vec::new();
    for raw_pk in pks {
        match ed25519::PublicKey::decode(raw_pk.as_slice()) {
            Ok(pk) => keys.push(pk),
            Err(_) => return false,
        }
    }
    let Ok(validators) = keys.into_iter().try_collect::<Set<_>>() else {
        return false;
    };
    let ns = union(engine::APPLICATION_NAMESPACE, b"_CONSENSUS");
    let Ok(scheme) = WeightedScheme::verifier(&ns, validators, weights.to_vec()) else {
        return false;
    };
    let Some(f) = crate::scheme::decode_finalization(&scheme, raw) else {
        // LCERT wrapper: skip header, try tail as raw Finalization.
        if raw.len() > 5 + 1 + 8 + 8 + 32 + 4 && &raw[0..5] == crate::cert::MAGIC {
            let n = u32::from_be_bytes(
                raw[5 + 1 + 8 + 8 + 32..5 + 1 + 8 + 8 + 32 + 4]
                    .try_into()
                    .unwrap(),
            ) as usize;
            let skip = 5 + 1 + 8 + 8 + 32 + 4 + n * (4 + 64) + 4;
            if skip < raw.len() {
                return verify_finalization_bytes(pks, weights, &raw[skip..]);
            }
        }
        return false;
    };
    let mut seed = [0u8; 32];
    let _ = SysRng.try_fill_bytes(&mut seed);
    let mut rng = StdRng::from_seed(seed);
    f.verify(&mut rng, &scheme, &Sequential)
}
