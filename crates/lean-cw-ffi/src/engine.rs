use crate::{
    automaton::{recv_payloads, App, Scheme},
    callbacks::RawCallbacks,
};
use commonware_codec::DecodeExt;
use commonware_consensus::{
    simplex::{self, elector::RoundRobin, Floor, ForwardingPolicy},
    types::{Epoch, ViewDelta},
};
use commonware_cryptography::{ed25519, Hasher, Sha256, Signer};
use commonware_p2p::{
    authenticated::{self, discovery},
    Ingress, Manager as _,
};
use commonware_parallel::Sequential;
use commonware_runtime::{
    buffer::paged::CacheRef, tokio, Quota, Runner as _, Spawner, Supervisor as _,
};
use commonware_utils::{channel::oneshot, union, NZU16, NZU32, NZUsize, TryCollect, ordered::Set};
use std::{
    net::{SocketAddr, ToSocketAddrs},
    str::FromStr,
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Mutex,
    },
    time::Duration,
};

pub const APPLICATION_NAMESPACE: &[u8] = b"_LEAN_CW_SIMPLEX";

pub static RUNNING: AtomicBool = AtomicBool::new(false);
pub static HEIGHT: AtomicU64 = AtomicU64::new(0);
pub static EPOCH: AtomicU64 = AtomicU64::new(0);
static STOP: Mutex<Option<oneshot::Sender<()>>> = Mutex::new(None);

#[derive(Clone)]
pub struct StartCfg {
    pub private_key: [u8; 32],
    pub listen: String,
    pub bootstrappers: String,
    pub storage_dir: String,
    pub namespace: String,
    pub participants: Vec<[u8; 32]>,
    pub epoch: u64,
}

pub fn genesis_digest(epoch: u64) -> commonware_cryptography::sha256::Digest {
    let ep = epoch.to_be_bytes();
    Sha256::hash(&[b"lean-cw-ffi-genesis", &ep])
}

pub fn parse_participants(bytes: &[u8]) -> Result<Vec<[u8; 32]>, String> {
    if bytes.len() % 32 != 0 {
        return Err(format!(
            "participants length {} is not a multiple of 32",
            bytes.len()
        ));
    }
    Ok(bytes
        .chunks_exact(32)
        .map(|c| {
            let mut a = [0u8; 32];
            a.copy_from_slice(c);
            a
        })
        .collect())
}

fn decode_pk(raw: &[u8; 32]) -> Result<ed25519::PublicKey, String> {
    ed25519::PublicKey::decode(raw.as_slice()).map_err(|e| format!("public key: {e}"))
}

fn decode_sk(raw: &[u8; 32]) -> Result<ed25519::PrivateKey, String> {
    ed25519::PrivateKey::decode(raw.as_slice()).map_err(|e| format!("private key: {e}"))
}


fn parse_bind_public(spec: &str) -> Result<(SocketAddr, SocketAddr), String> {
    let spec = spec.trim();
    if spec.is_empty() {
        let sa: SocketAddr = "0.0.0.0:26656".parse().unwrap();
        return Ok((sa, sa));
    }
    let (bind_s, pub_s) = match spec.split_once('|') {
        Some((a, b)) => (a.trim(), b.trim()),
        None => (spec, spec),
    };
    Ok((parse_sock(bind_s)?, parse_sock(pub_s)?))
}

fn parse_sock(s: &str) -> Result<SocketAddr, String> {
    if s.is_empty() {
        return "0.0.0.0:26656"
            .parse()
            .map_err(|e| format!("default listen: {e}"));
    }
    if let Ok(sa) = SocketAddr::from_str(s) {
        return Ok(sa);
    }
    let mut addrs: Vec<SocketAddr> = s
        .to_socket_addrs()
        .map_err(|e| format!("addr {s}: {e}"))?
        .collect();
    if addrs.is_empty() {
        return Err(format!("addr {s}: unresolved"));
    }
    addrs.sort_by_key(|a| a.is_ipv6());
    Ok(addrs.remove(0))
}

fn parse_bootstrappers(
    spec: &str,
) -> Result<Vec<(ed25519::PublicKey, Ingress)>, String> {
    let mut out = Vec::new();
    if spec.is_empty() {
        return Ok(out);
    }
    for part in spec.split(',') {
        let part = part.trim();
        if part.is_empty() {
            continue;
        }
        let (hex_pk, addr) = part
            .split_once('@')
            .ok_or_else(|| format!("bootstrapper {part} wants pkhex@host:port"))?;
        let pk_bytes = hex_decode(hex_pk)?;
        if pk_bytes.len() != 32 {
            return Err(format!("bootstrapper pk must be 32 bytes, got {}", pk_bytes.len()));
        }
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&pk_bytes);
        let pk = decode_pk(&arr)?;
        let sock = addr
            .to_socket_addrs()
            .map_err(|e| format!("bootstrapper addr {addr}: {e}"))?
            .next()
            .ok_or_else(|| format!("bootstrapper addr {addr}: unresolved"))?;
        out.push((pk, sock.into()));
    }
    Ok(out)
}

fn hex_decode(s: &str) -> Result<Vec<u8>, String> {
    let s = s.trim();
    if s.len() % 2 != 0 {
        return Err("odd hex length".into());
    }
    (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&s[i..i + 2], 16).map_err(|e| e.to_string()))
        .collect()
}

/// Block this thread on the commonware tokio runner until stop.
pub fn run_blocking(cfg: StartCfg, cb: RawCallbacks) -> Result<(), String> {
    let signer = decode_sk(&cfg.private_key)?;
    let (listen, public) = parse_bind_public(&cfg.listen)?;

    let mut raw_pks = cfg.participants.clone();
    if let Some(bytes) = cb.participants(cfg.epoch) {
        match parse_participants(&bytes) {
            Ok(p) if !p.is_empty() => raw_pks = p,
            Ok(_) => {}
            Err(e) => tracing::warn!(%e, "participants callback"),
        }
    }
    let mut pks: Vec<ed25519::PublicKey> = Vec::new();
    for raw in &raw_pks {
        pks.push(decode_pk(raw)?);
    }
    if pks.is_empty() {
        pks.push(signer.public_key());
    }
    let validators: Set<_> = pks
        .into_iter()
        .try_collect()
        .map_err(|_| "participant public keys must be unique")?;
    if validators.position(&signer.public_key()).is_none() {
        return Err("private key is not in participants".into());
    }
    if RUNNING.swap(true, Ordering::SeqCst) {
        return Err("lean-cw-ffi already running".into());
    }
    EPOCH.store(cfg.epoch, Ordering::SeqCst);

    let bootstrappers = parse_bootstrappers(&cfg.bootstrappers)?;
    let max_peers_per_set = authenticated::peer_set_limit(&validators, &signer.public_key());
    let storage = if cfg.storage_dir.is_empty() {
        "/tmp/lean-cw-ffi".to_string()
    } else {
        cfg.storage_dir.clone()
    };
    let ns = if cfg.namespace.is_empty() {
        APPLICATION_NAMESPACE.to_vec()
    } else {
        cfg.namespace.as_bytes().to_vec()
    };

    let runtime_cfg = tokio::Config::new().with_storage_directory(storage);
    let executor = tokio::Runner::new(runtime_cfg);

    let p2p_cfg = discovery::Config::local(
        signer.clone(),
        &union(&ns, b"_P2P"),
        listen,
        public,
        bootstrappers,
        max_peers_per_set,
        4 * 1024 * 1024,
    );

    let (stop_tx, stop_rx) = oneshot::channel::<()>();
    *STOP.lock().unwrap() = Some(stop_tx);

    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        executor.start(async move |context| {
            let (mut network, mut oracle) =
                discovery::Network::new(context.child("network"), p2p_cfg);
            oracle.track(0, validators.clone());

            let message_rate = Quota::per_second(NZU32!(32));
            let (vote_sender, vote_receiver) = network.register(0, message_rate);
            let (certificate_sender, certificate_receiver) = network.register(1, message_rate);
            let (resolver_sender, resolver_receiver) = network.register(2, message_rate);
            let (payload_sender, payload_receiver) = network.register(3, message_rate);

            let consensus_ns = union(&ns, b"_CONSENSUS");
            let scheme = Scheme::signer(&consensus_ns, validators.clone(), signer.clone())
                .expect("private key must be in participants");

            let app = App::new(cb);
            app.set_payload_sender(payload_sender.clone());
            context.child("payloads").spawn({
                let app = app.clone();
                move |_ctx| async move {
                    recv_payloads(payload_receiver, app).await;
                }
            });

            let engine_cfg = simplex::Config {
                scheme,
                elector: RoundRobin::<Sha256>::default(),
                blocker: oracle,
                automaton: app.clone(),
                relay: app.clone(),
                reporter: app,
                partition: format!("lean-{}", cfg.epoch),
                mailbox_size: NZUsize!(1024),
                epoch: Epoch::new(cfg.epoch),
                floor: Floor::Genesis(genesis_digest(cfg.epoch)),
                replay_buffer: NZUsize!(1024 * 1024),
                write_buffer: NZUsize!(1024 * 1024),
                leader_timeout: Duration::from_millis(800),
                certification_timeout: Duration::from_millis(1600),
                timeout_retry: Duration::from_secs(4),
                fetch_timeout: Duration::from_secs(1),
                view_retention: ViewDelta::new(16),
                skip_timeout: Duration::from_secs(5),
                page_cache: CacheRef::from_pooler(&context, NZU16!(16_384), NZUsize!(10_000)),
                strategy: Sequential,
                forwarding: ForwardingPolicy::SilentLeader,
                track_historical_votes: false,
            };
            let engine = simplex::Engine::new(context.child("engine"), engine_cfg);

            network.start();
            engine.start(
                (vote_sender, vote_receiver),
                (certificate_sender, certificate_receiver),
                (resolver_sender, resolver_receiver),
            );

            tracing::info!(%listen, "lean-cw-ffi simplex engine started");
            let _ = stop_rx.await;
            tracing::info!("lean-cw-ffi simplex engine stopping");
        });
    }));

    RUNNING.store(false, Ordering::SeqCst);
    *STOP.lock().unwrap() = None;
    result.map_err(|_| "lean-cw-ffi engine panicked".to_string())?;
    Ok(())
}

pub fn request_stop() {
    if let Some(tx) = STOP.lock().unwrap().take() {
        let _ = tx.send(());
    }
}

/// Seeded identity used in tests (`PrivateKey::from_seed`).
#[allow(dead_code)]
pub fn signer_from_seed(seed: u64) -> ed25519::PrivateKey {
    ed25519::PrivateKey::from_seed(seed)
}
