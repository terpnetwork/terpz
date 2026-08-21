package cwffi

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sync"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

// EngineApp is the ABCI subset Commonware Automaton/Reporter calls.
type EngineApp interface {
	Info(*abci.RequestInfo) (*abci.ResponseInfo, error)
	PrepareProposal(*abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error)
	ProcessProposal(*abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error)
	FinalizeBlock(*abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error)
	Commit() (*abci.ResponseCommit, error)
	CheckTx(*abci.RequestCheckTx) (*abci.ResponseCheckTx, error)
}

// Driver maps simplex Automaton/Reporter onto ABCI Prepare/Process/Finalize.
// Dummy DSTW still fails in Process. JOIN/LEAV stay LNPR injects.
type StoreQuery func(path string, data []byte) []byte

type committedBlock struct {
	payload []byte
	cert    []byte
}

type Driver struct {
	mu          sync.Mutex
	app         EngineApp
	proposer    []byte
	chainID     string
	next        int64
	lastCert    LeanCert
	lastCertRaw []byte
	lastPks     []byte
	lastWeights []uint64
	mempool     [][]byte
	committed   map[int64]committedBlock
	storeQuery  StoreQuery
	onCommit    func(int64)
	conflicts   int
	verifyCert  func(pks []byte, weights []uint64, cert []byte) bool
}

func NewDriver(app EngineApp, chainID string, proposer []byte) *Driver {
	d := &Driver{app: app, chainID: chainID, proposer: append([]byte(nil), proposer...)}
	if info, err := app.Info(&abci.RequestInfo{}); err == nil && info != nil {
		if info.LastBlockHeight < 1 {
			d.next = 1
		} else {
			d.next = info.LastBlockHeight + 1
		}
	} else {
		d.next = 1
	}
	return d
}

func (d *Driver) SetStoreQuery(q StoreQuery) {
	d.mu.Lock()
	d.storeQuery = q
	d.mu.Unlock()
}

func (d *Driver) SetOnCommit(fn func(int64)) {
	d.mu.Lock()
	d.onCommit = fn
	d.mu.Unlock()
}

func (d *Driver) SetVerifyCert(fn func(pks []byte, weights []uint64, cert []byte) bool) {
	d.mu.Lock()
	d.verifyCert = fn
	d.mu.Unlock()
}

func (d *Driver) LastCertificate() LeanCert {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastCert
}

func (d *Driver) LastCertificateRaw() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.lastCertRaw...)
}

func (d *Driver) LastParticipants() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.lastPks...)
}

func (d *Driver) LastWeights() []uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]uint64, len(d.lastWeights))
	copy(out, d.lastWeights)
	return out
}

func (d *Driver) Height() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	if info, err := d.app.Info(&abci.RequestInfo{}); err == nil && info != nil {
		return info.LastBlockHeight
	}
	return d.next - 1
}

func txKey(tx []byte) [32]byte {
	return sha256.Sum256(tx)
}

func (d *Driver) Propose(epoch, view uint64, _parent []byte) (digest, payload []byte, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	h := d.next
	if h < 1 {
		h = 1
	}
	txs := make([][]byte, 0, len(d.mempool))
	for _, tx := range d.mempool {
		txs = append(txs, append([]byte(nil), tx...))
	}
	req := &abci.RequestPrepareProposal{
		Height:          h,
		Time:            time.Now().UTC(),
		MaxTxBytes:      4 << 20,
		Txs:             txs,
		ProposerAddress: d.proposer,
	}
	resp, err := d.app.PrepareProposal(req)
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, fmt.Errorf("lean-cw: nil PrepareProposal")
	}
	p := Payload{Height: h, Txs: resp.Txs}
	raw := p.Encode()
	sum := sha256.Sum256(raw)
	return sum[:], raw, nil
}

func (d *Driver) Verify(_epoch, _view uint64, digest, payload []byte) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if containsDSTW(payload) {
		// Dummy DSTW is a reject fixture on Process/Certify/JOIN extras.
		// Process still decides LNPR; this is belt-and-suspenders.
	}
	p, err := DecodePayload(payload)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(payload)
	if len(digest) == 32 && string(sum[:]) != string(digest) {
		return false
	}
	resp, err := d.app.ProcessProposal(&abci.RequestProcessProposal{
		Txs:             p.Txs,
		Hash:            digest,
		Height:          p.Height,
		Time:            time.Now().UTC(),
		ProposerAddress: d.proposer,
	})
	if err != nil || resp == nil {
		return false
	}
	return resp.Status == abci.ResponseProcessProposal_ACCEPT
}

func (d *Driver) Certify(_epoch, _view uint64, _digest []byte) bool {
	return true
}

func (d *Driver) Report(kind uint32, epoch, view uint64, digest []byte) {
	if kind == 7 { // LEAN_CW_ACT_CONFLICT
		fmt.Fprintf(os.Stderr, "lean-cw: CONFLICTING activity kind=%d epoch=%d view=%d digest=%x\n", kind, epoch, view, digest)
		d.mu.Lock()
		d.conflicts++
		d.mu.Unlock()
	}
}

func (d *Driver) ConflictCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conflicts
}

func (d *Driver) Finalize(epoch, view uint64, digest, payload, cert []byte) {
	d.finalize(epoch, view, digest, payload, cert, false)
}

func (d *Driver) finalize(epoch, view uint64, digest, payload, cert []byte, remote bool) {
	d.mu.Lock()
	p, err := DecodePayload(payload)
	if err != nil {
		d.mu.Unlock()
		return
	}
	if d.next > 0 && p.Height < d.next {
		d.mu.Unlock()
		return
	}
	if d.next > 0 && p.Height != d.next {
		d.mu.Unlock()
		return
	}
	if len(digest) != 32 {
		sum := sha256.Sum256(payload)
		digest = sum[:]
	}

	pks, weights := d.participantsAndWeightsLocked(epoch)
	parsed, perr := ParseLeanCert(cert)
	if remote {
		if len(cert) == 0 {
			fmt.Fprintf(os.Stderr, "lean-cw: unsigned ApplyRemote forbidden\n")
			d.mu.Unlock()
			return
		}
		if d.verifyCert != nil && !d.verifyCert(pks, weights, cert) {
			fmt.Fprintf(os.Stderr, "lean-cw: catch-up certificate verify failed\n")
			d.mu.Unlock()
			return
		}
	}
	if perr == nil && len(parsed.Signers) > 0 {
		sw := signedWeight(parsed, weights)
		tw := totalWeight(weights)
		if tw > 0 && !MeetsWeight(sw, tw) {
			fmt.Fprintf(os.Stderr, "lean-cw: certificate weight %d/%d below 2/3; refusing Commit\n", sw, tw)
			d.mu.Unlock()
			return
		}
	}

	commit := lastCommitFromCert(parsed, pks, weights)
	_, err = d.app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Txs:               p.Txs,
		Hash:              digest,
		Height:            p.Height,
		Time:              time.Now().UTC(),
		ProposerAddress:   d.proposer,
		DecidedLastCommit: commit,
	})
	if err != nil {
		d.mu.Unlock()
		return
	}
	if _, err := d.app.Commit(); err != nil {
		d.mu.Unlock()
		return
	}
	included := make(map[[32]byte]struct{}, len(p.Txs))
	for _, tx := range p.Txs {
		included[txKey(tx)] = struct{}{}
	}
	kept := d.mempool[:0]
	for _, tx := range d.mempool {
		if _, ok := included[txKey(tx)]; !ok {
			kept = append(kept, tx)
		}
	}
	d.mempool = kept
	if perr == nil {
		d.lastCert = parsed
		if len(parsed.Raw) > 0 {
			d.lastCertRaw = append([]byte(nil), parsed.Raw...)
		} else {
			d.lastCertRaw = append([]byte(nil), cert...)
		}
	} else if len(cert) > 0 {
		d.lastCertRaw = append([]byte(nil), cert...)
	}
	d.lastPks = append([]byte(nil), pks...)
	d.lastWeights = append([]uint64(nil), weights...)
	if d.committed == nil {
		d.committed = make(map[int64]committedBlock)
	}
	d.committed[p.Height] = committedBlock{
		payload: append([]byte(nil), payload...),
		cert:    append([]byte(nil), cert...),
	}
	d.next = p.Height + 1
	setEngineHeight(uint64(p.Height))
	cb := d.onCommit
	h := p.Height
	d.mu.Unlock()
	if cb != nil {
		cb(h)
	}
}

// ApplyRemote executes a remote payload only after a certificate verifies.
func (d *Driver) ApplyRemote(payload, cert []byte) {
	if len(payload) == 0 || len(cert) == 0 {
		return
	}
	sum := sha256.Sum256(payload)
	d.finalize(0, 0, sum[:], payload, cert, true)
}

func (d *Driver) PayloadAt(h int64) (payload []byte, height int64, cert []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if h <= 0 {
		h = d.next - 1
	}
	if h < 1 {
		return nil, 0, append([]byte(nil), d.lastCertRaw...)
	}
	if d.committed != nil {
		if b, ok := d.committed[h]; ok {
			return append([]byte(nil), b.payload...), h, append([]byte(nil), b.cert...)
		}
	}
	return nil, h, append([]byte(nil), d.lastCertRaw...)
}

func (d *Driver) CheckTx(tx []byte) (*abci.ResponseCheckTx, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	resp, err := d.app.CheckTx(&abci.RequestCheckTx{Tx: tx, Type: abci.CheckTxType_New})
	if err != nil {
		return resp, err
	}
	if resp != nil && resp.Code == 0 && len(tx) > 0 {
		k := txKey(tx)
		for _, existing := range d.mempool {
			if txKey(existing) == k {
				return resp, nil
			}
		}
		d.mempool = append(d.mempool, append([]byte(nil), tx...))
	}
	return resp, nil
}

func (d *Driver) Info() (*abci.ResponseInfo, error) {
	return d.app.Info(&abci.RequestInfo{})
}

func (d *Driver) participantsAndWeightsLocked(epoch uint64) ([]byte, []uint64) {
	q := d.storeQuery
	if q == nil {
		return append([]byte(nil), d.lastPks...), append([]uint64(nil), d.lastWeights...)
	}
	pks, w := BondedParticipantsWeights(q, epoch)
	if len(pks) == 0 {
		return append([]byte(nil), d.lastPks...), append([]uint64(nil), d.lastWeights...)
	}
	return pks, w
}

func lastCommitFromCert(c LeanCert, pks []byte, weights []uint64) abci.CommitInfo {
	votes := make([]abci.VoteInfo, 0, len(c.Signers))
	for _, s := range c.Signers {
		off := int(s.Index) * 32
		if off < 0 || off+32 > len(pks) {
			continue
		}
		pk := pks[off : off+32]
		power := int64(0)
		if int(s.Index) < len(weights) {
			power = int64(weights[s.Index])
		}
		if power <= 0 {
			power = 1
		}
		addr := ed25519.PubKey(pk).Address()
		if isZeroSig(s.Signature) {
			continue
		}
		votes = append(votes, abci.VoteInfo{
			Validator: abci.Validator{
				Address: addr,
				Power:   power,
			},
			BlockIdFlag: cmtproto.BlockIDFlagCommit,
		})
	}
	return abci.CommitInfo{Round: 0, Votes: votes}
}

func isZeroSig(sig []byte) bool {
	if len(sig) == 0 {
		return true
	}
	for _, b := range sig {
		if b != 0 {
			return false
		}
	}
	return true
}
