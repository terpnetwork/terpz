package cwffi

import (
	"context"
	"crypto/sha256"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
)

type fakeApp struct {
	height   int64
	prepare  func(*abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error)
	process  func(*abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error)
	finalize int
	commit   int
}

func (f *fakeApp) Info(*abci.RequestInfo) (*abci.ResponseInfo, error) {
	return &abci.ResponseInfo{LastBlockHeight: f.height}, nil
}
func (f *fakeApp) Query(context.Context, *abci.RequestQuery) (*abci.ResponseQuery, error) {
	return &abci.ResponseQuery{}, nil
}
func (f *fakeApp) CheckTx(*abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	return &abci.ResponseCheckTx{}, nil
}
func (f *fakeApp) InitChain(*abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	return &abci.ResponseInitChain{}, nil
}
func (f *fakeApp) PrepareProposal(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	if f.prepare != nil {
		return f.prepare(req)
	}
	return &abci.ResponsePrepareProposal{Txs: req.Txs}, nil
}
func (f *fakeApp) ProcessProposal(req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	if f.process != nil {
		return f.process(req)
	}
	return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
}
func (f *fakeApp) FinalizeBlock(*abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	f.finalize++
	return &abci.ResponseFinalizeBlock{}, nil
}
func (f *fakeApp) ExtendVote(context.Context, *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
	return &abci.ResponseExtendVote{}, nil
}
func (f *fakeApp) VerifyVoteExtension(*abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	return &abci.ResponseVerifyVoteExtension{}, nil
}
func (f *fakeApp) Commit() (*abci.ResponseCommit, error) {
	f.commit++
	f.height++
	return &abci.ResponseCommit{}, nil
}
func (f *fakeApp) ListSnapshots(*abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error) {
	return &abci.ResponseListSnapshots{}, nil
}
func (f *fakeApp) OfferSnapshot(*abci.RequestOfferSnapshot) (*abci.ResponseOfferSnapshot, error) {
	return &abci.ResponseOfferSnapshot{}, nil
}
func (f *fakeApp) LoadSnapshotChunk(*abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error) {
	return &abci.ResponseLoadSnapshotChunk{}, nil
}
func (f *fakeApp) ApplySnapshotChunk(*abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error) {
	return &abci.ResponseApplySnapshotChunk{}, nil
}

func TestProposeCallsPrepare(t *testing.T) {
	app := &fakeApp{}
	called := false
	app.prepare = func(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		called = true
		if req.Height != 1 {
			t.Fatalf("height %d", req.Height)
		}
		return &abci.ResponsePrepareProposal{Txs: [][]byte{[]byte("LNPR")}}, nil
	}
	d := NewDriver(app, "lean", []byte{1, 2, 3})
	digest, payload, err := d.Propose(0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("PrepareProposal not called")
	}
	sum := sha256.Sum256(payload)
	if string(digest) != string(sum[:]) {
		t.Fatal("digest")
	}
	got, err := DecodePayload(payload)
	if err != nil || len(got.Txs) != 1 {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestVerifyRejectsProcessReject(t *testing.T) {
	app := &fakeApp{}
	app.process = func(*abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
	}
	d := NewDriver(app, "lean", nil)
	p := Payload{Height: 1, Txs: [][]byte{[]byte("DSTW")}}
	raw := p.Encode()
	sum := sha256.Sum256(raw)
	if d.Verify(0, 1, sum[:], raw) {
		t.Fatal("Dummy/Process REJECT must fail verify")
	}
}

func TestFinalizeCallsCommit(t *testing.T) {
	app := &fakeApp{}
	d := NewDriver(app, "lean", nil)
	p := Payload{Height: 1, Txs: [][]byte{[]byte("LNPR")}}
	raw := p.Encode()
	sum := sha256.Sum256(raw)
	d.Finalize(0, 1, sum[:], raw, nil)
	if app.finalize != 1 || app.commit != 1 {
		t.Fatalf("finalize=%d commit=%d", app.finalize, app.commit)
	}
}

func TestApplyRemoteIdempotent(t *testing.T) {
	app := &fakeApp{}
	d := NewDriver(app, "lean", nil)
	d.SetVerifyCert(func([]byte, []uint64, []byte) bool { return true })
	p := Payload{Height: 1, Txs: [][]byte{[]byte("LNPR")}}
	raw := p.Encode()
	cert := []byte("LCERT") // non-empty; verify hook accepts
	d.ApplyRemote(raw, cert)
	d.ApplyRemote(raw, cert)
	if app.finalize != 1 || app.commit != 1 {
		t.Fatalf("finalize=%d commit=%d", app.finalize, app.commit)
	}
	if d.Height() != 1 {
		t.Fatalf("height %d", d.Height())
	}
	got, h, _ := d.PayloadAt(1)
	if h != 1 || len(got) == 0 {
		t.Fatalf("payload h=%d n=%d", h, len(got))
	}
}

func TestApplyRemoteUnsignedForbidden(t *testing.T) {
	app := &fakeApp{}
	d := NewDriver(app, "lean", nil)
	p := Payload{Height: 1, Txs: [][]byte{[]byte("LNPR")}}
	d.ApplyRemote(p.Encode(), nil)
	if app.finalize != 0 || app.commit != 0 {
		t.Fatalf("unsigned ApplyRemote must not Commit, finalize=%d commit=%d", app.finalize, app.commit)
	}
}

func TestReportConflictFailsLoud(t *testing.T) {
	app := &fakeApp{}
	d := NewDriver(app, "lean", nil)
	d.Report(7, 1, 2, make([]byte, 32))
	if d.ConflictCount() != 1 {
		t.Fatalf("conflicts %d", d.ConflictCount())
	}
}

func TestBondedParticipantsEmpty(t *testing.T) {
	if BondedParticipants(nil, 0) != nil {
		t.Fatal("nil query")
	}
	if BondedParticipants(func(string, []byte) []byte { return nil }, 1) != nil {
		t.Fatal("empty store")
	}
}
