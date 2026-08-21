use crate::callbacks::{RawCallbacks, DIGEST_LEN};
use commonware_actor::Feedback;
use commonware_consensus::{
    simplex::{
        types::{Activity, Context},
        Plan,
    },
    types::{Epoch, View},
    Automaton as Au, CertifiableAutomaton as CAu, Epochable, Relay as Re, Reporter as Rp, Viewable,
};
use commonware_cryptography::{ed25519::PublicKey, sha256::Digest, Hasher, Sha256};
use commonware_p2p::{Recipients, Sender};
use commonware_utils::channel::oneshot;
use std::{
    collections::HashMap,
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};

pub type Scheme = crate::scheme::WeightedScheme;

#[derive(Clone)]
pub struct App<S>
where
    S: Sender<PublicKey = PublicKey> + Clone + Send + 'static,
{
    cb: RawCallbacks,
    store: Arc<Mutex<HashMap<Digest, Vec<u8>>>>,
    payload_tx: Arc<Mutex<Option<S>>>,
}

impl<S> App<S>
where
    S: Sender<PublicKey = PublicKey> + Clone + Send + 'static,
{
    pub fn new(cb: RawCallbacks) -> Self {
        Self {
            cb,
            store: Arc::new(Mutex::new(HashMap::new())),
            payload_tx: Arc::new(Mutex::new(None)),
        }
    }

    pub fn set_payload_sender(&self, sender: S) {
        *self.payload_tx.lock().unwrap() = Some(sender);
    }

    pub fn insert_payload(&self, digest: Digest, payload: Vec<u8>) {
        self.store.lock().unwrap().insert(digest, payload);
    }

    pub fn get_payload(&self, digest: &Digest) -> Option<Vec<u8>> {
        self.store.lock().unwrap().get(digest).cloned()
    }

    fn digest_bytes(d: &Digest) -> [u8; DIGEST_LEN] {
        let mut out = [0u8; DIGEST_LEN];
        out.copy_from_slice(d.as_ref());
        out
    }

    fn digest_from(bytes: [u8; DIGEST_LEN]) -> Digest {
        Digest(bytes)
    }
}

impl<S> Au for App<S>
where
    S: Sender<PublicKey = PublicKey> + Clone + Send + 'static,
{
    type Digest = Digest;
    type Context = Context<Self::Digest, PublicKey>;

    async fn propose(&mut self, context: Self::Context) -> oneshot::Receiver<Self::Digest> {
        let (response, receiver) = oneshot::channel();
        let cb = self.cb;
        let store = self.store.clone();
        let epoch = context.epoch().get();
        let view = context.view().get();
        let parent = Self::digest_bytes(&context.parent.1);
        thread::spawn(move || {
            let Some((digest_raw, payload)) = cb.propose(epoch, view, &parent) else {
                return;
            };
            let digest = App::<S>::digest_from(digest_raw);
            if payload.is_empty() {
                let hashed = Sha256::hash(&[&digest_raw]);
                if hashed.as_ref() != digest.as_ref() {
                    tracing::warn!("propose returned digest without payload");
                }
            } else {
                let hashed = Sha256::hash(&[&payload]);
                if hashed.as_ref() != digest.as_ref() {
                    tracing::warn!("propose digest mismatch; using hash of payload");
                    store.lock().unwrap().insert(hashed, payload);
                    let _ = response.send(hashed);
                    return;
                }
                store.lock().unwrap().insert(digest, payload);
            }
            let _ = response.send(digest);
        });
        receiver
    }

    async fn verify(
        &mut self,
        context: Self::Context,
        payload: Self::Digest,
    ) -> oneshot::Receiver<bool> {
        let (response, receiver) = oneshot::channel();
        let cb = self.cb;
        let store = self.store.clone();
        let epoch = context.epoch().get();
        let view = context.view().get();
        thread::spawn(move || {
            let digest_raw = App::<S>::digest_bytes(&payload);
            loop {
                let bytes = store.lock().unwrap().get(&payload).cloned();
                if let Some(bytes) = bytes {
                    let ok = cb.verify(epoch, view, &digest_raw, &bytes);
                    let _ = response.send(ok);
                    return;
                }
                thread::sleep(Duration::from_millis(50));
            }
        });
        receiver
    }
}

impl<S> CAu for App<S>
where
    S: Sender<PublicKey = PublicKey> + Clone + Send + 'static,
{
    async fn certify(
        &mut self,
        round: commonware_consensus::types::Round,
        payload: Self::Digest,
    ) -> oneshot::Receiver<bool> {
        let (response, receiver) = oneshot::channel();
        let cb = self.cb;
        let store = self.store.clone();
        let epoch = round.epoch().get();
        let view = round.view().get();
        let digest_raw = Self::digest_bytes(&payload);
        thread::spawn(move || {
            let bytes = store
                .lock()
                .unwrap()
                .get(&payload)
                .cloned()
                .unwrap_or_default();
            if crate::cert::contains_dstw(&bytes) {
                tracing::warn!("certify rejected Dummy DSTW");
                let _ = response.send(false);
                return;
            }
            let ok = cb.certify(epoch, view, &digest_raw);
            let _ = response.send(ok);
        });
        receiver
    }
}

impl<S> Re for App<S>
where
    S: Sender<PublicKey = PublicKey> + Clone + Send + 'static,
{
    type Digest = Digest;
    type PublicKey = PublicKey;
    type Plan = Plan<PublicKey>;

    fn broadcast(&mut self, payload: Self::Digest, plan: Self::Plan) -> Feedback {
        let Some(bytes) = self.get_payload(&payload) else {
            tracing::debug!("relay: payload missing for digest, skip broadcast");
            return Feedback::Ok;
        };
        let mut guard = self.payload_tx.lock().unwrap();
        let Some(sender) = guard.as_mut() else {
            return Feedback::Ok;
        };
        let recipients = match plan {
            Plan::Propose { .. } => Recipients::All,
            Plan::Forward { recipients, .. } => recipients,
        };
        let _ = sender.send(recipients, bytes, false);
        Feedback::Ok
    }
}

impl<S> Rp for App<S>
where
    S: Sender<PublicKey = PublicKey> + Clone + Send + 'static,
{
    type Activity = Activity<Scheme, Digest>;

    fn report(&mut self, activity: Self::Activity) -> Feedback {
        let (kind, epoch, view, digest) = activity_parts(&activity);
        self.cb.report(kind, epoch.get(), view.get(), &digest);
        if kind == 6 {
            crate::engine::HEIGHT.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
            let (lcert, raw) = match &activity {
                Activity::Finalization(v) => (
                    crate::cert::encode_lcert(v),
                    crate::scheme::encode_finalization(v),
                ),
                _ => (Vec::new(), Vec::new()),
            };
            crate::engine::persist_finalization(raw, lcert.clone());
            let payload = self.get_payload(&Digest(digest)).unwrap_or_default();
            self.cb
                .finalize(epoch.get(), view.get(), &digest, &payload, &lcert);
        }
        Feedback::Ok
    }
}

fn activity_parts(activity: &Activity<Scheme, Digest>) -> (u32, Epoch, View, [u8; DIGEST_LEN]) {
    use commonware_consensus::{Epochable, Viewable};
    let epoch = activity.epoch();
    let view = activity.view();
    let zero = [0u8; DIGEST_LEN];
    match activity {
        Activity::Notarize(v) => (1, epoch, view, copy_digest(&v.proposal.payload)),
        Activity::Notarization(v) => (2, epoch, view, copy_digest(&v.proposal.payload)),
        Activity::Certification(v) => (2, epoch, view, copy_digest(&v.proposal.payload)),
        Activity::Nullify(_) => (3, epoch, view, zero),
        Activity::Nullification(_) => (4, epoch, view, zero),
        Activity::Finalize(v) => (5, epoch, view, copy_digest(&v.proposal.payload)),
        Activity::Finalization(v) => (6, epoch, view, copy_digest(&v.proposal.payload)),
        Activity::ConflictingNotarize(_)
        | Activity::ConflictingFinalize(_)
        | Activity::NullifyFinalize(_) => (7, epoch, view, zero),
    }
}

fn copy_digest(d: &Digest) -> [u8; DIGEST_LEN] {
    let mut out = [0u8; DIGEST_LEN];
    out.copy_from_slice(d.as_ref());
    out
}

/// Receive payload bytes from peers into the digest store.
pub async fn recv_payloads<R, S>(mut rx: R, app: App<S>)
where
    R: commonware_p2p::Receiver<PublicKey = PublicKey>,
    S: Sender<PublicKey = PublicKey> + Clone + Send + 'static,
{
    loop {
        match rx.recv().await {
            Ok((_pk, buf)) => {
                let bytes: &[u8] = buf.as_ref();
                let digest = Sha256::hash(&[bytes]);
                app.insert_payload(digest, bytes.to_vec());
            }
            Err(err) => {
                tracing::debug!(?err, "payload channel closed");
                break;
            }
        }
    }
}
