package keeper

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// Tx order when composing with hashmerchant:
//
//	if txs[0] is HMVE (0x48 0x4D 0x56 0x45), LNPR is txs[1]
//	else LNPR is txs[0]
//
// Lean wraps hashmerchant Prepare/Process; it never SetPrepareProposal alone.

type PrepareHandler func(*abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error)
type ProcessHandler func(*abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error)

// WrapPrepareProposal runs the inner (hashmerchant) handler first, then injects LNPR.
func (k *Keeper) WrapPrepareProposal(inner PrepareHandler) PrepareHandler {
	return func(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		var txs [][]byte
		if inner != nil {
			resp, err := inner(req)
			if err != nil {
				return nil, err
			}
			if resp != nil {
				txs = resp.Txs
			}
		} else {
			txs = append([][]byte(nil), req.Txs...)
		}
		period := types.PeriodFromHeight(req.Height)
		lnpr := k.buildLNPR(period)
		txs = injectLNPR(txs, lnpr)
		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}

// WrapProcessProposal: invalid if required LNPR omitted OR VerifyDummy fails.
// Then calls inner (hashmerchant) ProcessProposal.
func (k *Keeper) WrapProcessProposal(inner ProcessHandler) ProcessHandler {
	return func(req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		if err := k.checkLNPR(req.Height, req.Txs); err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}
		if inner != nil {
			return inner(req)
		}
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	}
}

func (k *Keeper) checkLNPR(height int64, txs [][]byte) error {
	blob, idx, ok := FindLNPR(txs)
	if !ok {
		if k.RequireLNPR {
			return fmt.Errorf("leanval: required LNPR omitted")
		}
		return nil
	}
	if idx != expectedLNPRIndex(txs) {
		return fmt.Errorf("leanval: LNPR at wrong index %d (want %d; HMVE-first compose)", idx, expectedLNPRIndex(txs))
	}
	want := types.PeriodFromHeight(height)
	if blob.Period != want {
		return fmt.Errorf("leanval: LNPR period %d != height period %d", blob.Period, want)
	}
	return k.VerifyLNPR(blob)
}

// buildLNPR encodes Dummy proofs for the committed BondedSet only.
// Disk lean-pending.json / LEANVAL_PENDING is not admission: two honest
// replicas with the same app hash must propose the same set.
func (k *Keeper) buildLNPR(period uint64) []byte {
	set := k.BondedSetOrCarry(period)
	roots := k.LastObjectRoots()
	subs := make([]types.SubjectProof, 0, len(set))
	for _, s := range set {
		subs = append(subs, types.SubjectProof{
			Subject: s.Subject,
			Weight:  s.Weight,
			Proof:   DummyStwoProveBoundRoots(period, s.Subject, s.Weight, roots),
		})
	}
	return types.EncodeLNPR(types.LNPRBlob{Period: period, Subjects: subs})
}

// InjectLocalProofs is how a proposer attaches dummy proofs before Prepare.
func (k *Keeper) InjectLocalProofs(period uint64, subjects []types.SubjectProof) []byte {
	return types.EncodeLNPR(types.LNPRBlob{Period: period, Subjects: subjects})
}

func injectLNPR(txs [][]byte, lnpr []byte) [][]byte {
	out := make([][]byte, 0, len(txs)+1)
	// Drop any existing LNPR so we own the inject slot.
	for _, tx := range txs {
		if types.HasPrefix(tx, types.PrefixLNPR) {
			continue
		}
		out = append(out, tx)
	}
	idx := expectedLNPRIndex(out)
	// insert at idx
	out = append(out, nil)
	copy(out[idx+1:], out[idx:])
	out[idx] = lnpr
	return out
}

func expectedLNPRIndex(txs [][]byte) int {
	if len(txs) > 0 && types.HasPrefix(txs[0], types.PrefixHMVE) {
		return 1
	}
	return 0
}

// FindLNPR returns the first LNPR blob and its index.
func FindLNPR(txs [][]byte) (types.LNPRBlob, int, bool) {
	for i, tx := range txs {
		if blob, ok := types.DecodeLNPR(tx); ok {
			return blob, i, true
		}
	}
	return types.LNPRBlob{}, -1, false
}

// ProcessInjectedLNPR applies an already-accepted proposal's LNPR in FinalizeBlock.
func (k *Keeper) ProcessInjectedLNPR(txs [][]byte) error {
	blob, _, ok := FindLNPR(txs)
	if !ok {
		if k.RequireLNPR {
			return fmt.Errorf("leanval: required LNPR omitted at finalize")
		}
		return nil
	}
	return k.ApplyLNPR(blob)
}
