package cwffi

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
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

type Driver struct {
	mu         sync.Mutex
	app        EngineApp
	proposer   []byte
	chainID    string
	next       int64
	committee  int
	lastSigs   int
	mempool    [][]byte
	committed  map[int64][]byte
	storeQuery StoreQuery
	onCommit   func(int64)
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

func (d *Driver) SetCommittee(n int) {
	if n < 0 {
		n = 0
	}
	d.mu.Lock()
	d.committee = n
	d.mu.Unlock()
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

func (d *Driver) LastCommitSigs() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lastSigs > 0 {
		return d.lastSigs
	}
	return d.committee
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
	_ = kind
	_ = epoch
	_ = view
	_ = digest
}

func (d *Driver) Finalize(_epoch, _view uint64, digest, payload []byte) {
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
	_, err = d.app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Txs:             p.Txs,
		Hash:            digest,
		Height:          p.Height,
		Time:            time.Now().UTC(),
		ProposerAddress: d.proposer,
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
	if d.committee > 0 {
		d.lastSigs = d.committee
	}
	if d.committed == nil {
		d.committed = make(map[int64][]byte)
	}
	d.committed[p.Height] = append([]byte(nil), payload...)
	d.next = p.Height + 1
	setEngineHeight(uint64(p.Height))
	cb := d.onCommit
	h := p.Height
	d.mu.Unlock()
	if cb != nil {
		cb(h)
	}
}

func (d *Driver) ApplyRemote(payload []byte) {
	if len(payload) == 0 {
		return
	}
	sum := sha256.Sum256(payload)
	d.Finalize(0, 0, sum[:], payload)
}

func (d *Driver) PayloadAt(h int64) (payload []byte, height int64, sigs int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if h <= 0 {
		h = d.next - 1
	}
	if h < 1 {
		return nil, 0, d.lastSigs
	}
	if d.committed != nil {
		payload = append([]byte(nil), d.committed[h]...)
	}
	sigs = d.lastSigs
	if sigs == 0 {
		sigs = d.committee
	}
	return payload, h, sigs
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
