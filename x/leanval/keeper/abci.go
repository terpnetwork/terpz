package keeper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		incoming := append([][]byte(nil), req.Txs...)
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
			txs = append([][]byte(nil), incoming...)
		}
		period := types.PeriodFromHeight(req.Height)
		for _, tx := range incoming {
			k.NoteMembershipTx(tx)
		}
		kept := make([][]byte, 0, len(txs))
		for _, tx := range txs {
			if types.IsMembershipTx(tx) {
				k.NoteMembershipTx(tx)
				continue
			}
			kept = append(kept, tx)
		}
		txs = kept
		// JOIN is admitted via LNPR subjects (ApplyLNPR), not as extra Finalize txs
		// (MembershipTx GetMsgs is empty and fails Deliver).
		lnpr := k.buildLNPR(period)
		blob, _, _ := FindLNPR([][]byte{lnpr})
		pending := len(k.PendingMembershipTxs())
		fmt.Fprintf(os.Stderr, "leanval: prepare h=%d pending=%d lnpr_subjects=%d req_txs=%d\n",
			req.Height, pending, len(blob.Subjects), len(incoming))
		writeLastPrepare(pending, len(blob.Subjects))
		txs = injectLNPR(txs, lnpr)
		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}

// WrapProcessProposal: invalid if required LNPR omitted OR VerifyDummy fails.
// Then calls inner (hashmerchant) ProcessProposal.
func (k *Keeper) WrapProcessProposal(inner ProcessHandler) ProcessHandler {
	return func(req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		if err := k.checkLNPR(req.Height, req.Txs); err != nil {
			fmt.Fprintf(os.Stderr, "leanval: Process REJECT on unverifiable LNPR: %v\n", err)
			writeLastProcess("REJECT", err.Error(), req.Height, req.Txs)
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}
		writeLastProcess("ACCEPT", "", req.Height, req.Txs)
		if inner != nil {
			resp, err := inner(req)
			if err != nil {
				return resp, err
			}
			if resp != nil && resp.Status == abci.ResponseProcessProposal_ACCEPT {
				fmt.Fprintf(os.Stderr, "leanval: Process ACCEPT\n")
			}
			return resp, nil
		}
		fmt.Fprintf(os.Stderr, "leanval: Process ACCEPT\n")
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

// buildLNPR encodes the roster this block certifies: committed bits plus
// queued JOIN/LEAV subjects. Pending files stay on the LNPR even if the
// resulting blob will fail Verify — Process REJECT is a stall, not a cue
// to propose genesis-only and look like progress.
func (k *Keeper) buildLNPR(period uint64) []byte {
	k.syncObjectRoots()
	set := k.DebugSubjectsFromBits()
	if len(set) == 0 {
		set = k.BondedSetOrCarry(period)
	}
	known := make(map[string]struct{}, len(set))
	for _, s := range set {
		known[string(s.Subject)] = struct{}{}
	}
	set = k.applyQueuedMembership(period, set)
	subs := make([]types.SubjectProof, 0, len(set))
	for _, s := range set {
		subs = append(subs, types.SubjectProof{Subject: s.Subject, Weight: s.Weight})
	}
	roots := k.LastObjectRoots()
	pairs := foldPairStrings(period, subs, roots)
	if fold, err := ProveSameStatementFold(pairs); err == nil && len(fold) > 0 {
		idx := 0
		for i, s := range subs {
			if _, ok := known[string(s.Subject)]; ok {
				idx = i
				break
			}
		}
		if len(subs) == 0 {
			subs = []types.SubjectProof{{Subject: types.DepositIndexBytes(0), Weight: 0, Proof: fold}}
		} else {
			subs[idx].Proof = fold
		}
	}
	// JOIN extras stay on the roster even if Dummy is closed (unverifiable).
	// Fold of current object roots does not prove them. Dummy extras bind
	// store-sourced roots (waist). Do not hide a stall with genesis-only.
	if k.AllowDummy {
		for i := range subs {
			if isFoldProof(subs[i].Proof) {
				continue
			}
			subs[i].Proof = DummyStwoProveBoundRoots(period, subs[i].Subject, subs[i].Weight, roots)
		}
	}
	return types.EncodeLNPR(types.LNPRBlob{Period: period, Subjects: subs})
}

// InjectLocalProofs is how a proposer attaches dummy proofs before Prepare.
func (k *Keeper) InjectLocalProofs(period uint64, subjects []types.SubjectProof) []byte {
	return types.EncodeLNPR(types.LNPRBlob{Period: period, Subjects: subjects})
}

func mergeMembershipFromComet(reqTxs, proposed [][]byte) [][]byte {
	out := append([][]byte(nil), proposed...)
	seen := map[string]struct{}{}
	for _, tx := range out {
		seen[string(tx)] = struct{}{}
	}
	for _, tx := range reqTxs {
		if !types.IsMembershipTx(tx) {
			continue
		}
		if _, ok := seen[string(tx)]; ok {
			continue
		}
		out = append(out, append([]byte(nil), tx...))
		seen[string(tx)] = struct{}{}
	}
	return out
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

type lastProcessFile struct {
	Status       string `json:"status"`
	Err          string `json:"err,omitempty"`
	Height       int64  `json:"height"`
	LNPRSubjects int    `json:"lnpr_subjects"`
	DummyExtras  int    `json:"dummy_extras"`
}

func writeLastProcess(status, errMsg string, height int64, txs [][]byte) {
	dir := membershipDir()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	blob, _, ok := FindLNPR(txs)
	nsubj, extras := 0, 0
	if ok {
		nsubj = len(blob.Subjects)
		for _, s := range blob.Subjects {
			if s.Weight > 0 && len(s.Proof) > 0 && !isFoldProof(s.Proof) {
				extras++
			}
		}
	}
	rec := lastProcessFile{Status: status, Err: errMsg, Height: height, LNPRSubjects: nsubj, DummyExtras: extras}
	bz, err := json.Marshal(rec)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "last-process"), append(bz, '\n'), 0o644)
}

func countSetBits(k *Keeper) int {
	n := 0
	for i := uint32(0); i < k.nextDepositIndex()+8; i++ {
		if k.BitIsSet(i) {
			n++
		}
	}
	return n
}

type lastApplyFile struct {
	Status       string `json:"status"`
	Err          string `json:"err,omitempty"`
	BitsBefore   int    `json:"bits_before"`
	BitsAfter    int    `json:"bits_after"`
	LNPRSubjects int    `json:"lnpr_subjects"`
}

func writeLastApply(status, errMsg string, before, after, nsubj int) {
	dir := membershipDir()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	bz, err := json.Marshal(lastApplyFile{Status: status, Err: errMsg, BitsBefore: before, BitsAfter: after, LNPRSubjects: nsubj})
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "last-apply"), append(bz), 0o644)
}
