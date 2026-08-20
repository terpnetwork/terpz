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
type Driver struct {
	mu       sync.Mutex
	app      EngineApp
	proposer []byte
	chainID  string
	next     int64
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

func (d *Driver) Height() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	if info, err := d.app.Info(&abci.RequestInfo{}); err == nil && info != nil {
		return info.LastBlockHeight
	}
	return d.next - 1
}

func (d *Driver) Propose(epoch, view uint64, _parent []byte) (digest, payload []byte, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	h := d.next
	if h < 1 {
		h = 1
	}
	req := &abci.RequestPrepareProposal{
		Height:          h,
		Time:            time.Now().UTC(),
		MaxTxBytes:      4 << 20,
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
	defer d.mu.Unlock()
	p, err := DecodePayload(payload)
	if err != nil {
		return
	}
	_, err = d.app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Txs:             p.Txs,
		Hash:            digest,
		Height:          p.Height,
		Time:            time.Now().UTC(),
		ProposerAddress: d.proposer,
	})
	if err != nil {
		return
	}
	if _, err := d.app.Commit(); err != nil {
		return
	}
	d.next = p.Height + 1
	setEngineHeight(uint64(p.Height))
}

func (d *Driver) CheckTx(tx []byte) (*abci.ResponseCheckTx, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.app.CheckTx(&abci.RequestCheckTx{Tx: tx, Type: abci.CheckTxType_New})
}

func (d *Driver) Info() (*abci.ResponseInfo, error) {
	return d.app.Info(&abci.RequestInfo{})
}
