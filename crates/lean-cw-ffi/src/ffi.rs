use crate::{
    callbacks::RawCallbacks,
    engine::{self, StartCfg, HEIGHT, EPOCH, RUNNING},
};
use std::{
    ffi::CStr,
    os::raw::c_char,
    slice,
    sync::atomic::Ordering,
    thread,
};

#[no_mangle]
pub extern "C" fn lean_cw_start(
    cfg: *const crate::lean_cw_cfg,
    cb: *const RawCallbacks,
) -> i32 {
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

    let start = StartCfg {
        private_key,
        listen,
        bootstrappers,
        storage_dir,
        namespace,
        participants,
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

    // Give the runner a moment to set RUNNING.
    for _ in 0..50 {
        if RUNNING.load(Ordering::SeqCst) {
            break;
        }
        thread::sleep(std::time::Duration::from_millis(20));
    }
    crate::LEAN_CW_OK
}

#[no_mangle]
pub extern "C" fn lean_cw_stop() -> i32 {
    engine::request_stop();
    crate::LEAN_CW_OK
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
    unsafe { CStr::from_ptr(p) }
        .to_string_lossy()
        .into_owned()
}

/// Called from Go finalize to publish committed height (view != height).
#[no_mangle]
pub extern "C" fn lean_cw_set_height(height: u64) {
    HEIGHT.store(height, Ordering::SeqCst);
}
