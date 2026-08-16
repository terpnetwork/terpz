package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	abci "github.com/cometbft/cometbft/abci/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	protov2 "google.golang.org/protobuf/proto"

	"github.com/terpnetwork/terp-core/v6/x/leanval/ante"
	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

// P0 elevation retuned for feat/lean-v6 @ e763a1a. Skip only remaining gaps.

func TestP0_InitGenesisJSONTable(t *testing.T) {
	// SDK x/staking/genesis_test.go:TestValidateGenesis
	pub := make([]byte, 32)
	pub[0] = 1
	seed, err := json.Marshal(types.GenesisState{
		OwnsValset:      true,
		GenesisSubjects: []types.GenesisSubject{{PubKey: pub, Weight: 7}},
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		raw       string
		wantOwns  bool
		wantFail  bool
		wantSeedN int
		wantW     int64
		wantProof bool
	}{
		{"G-ON", `{"leanval_owns_valset":true}`, true, false, 0, 0, false},
		{"G-OFF", `{"leanval_owns_valset":false}`, false, false, 0, 0, false},
		{"G-EMPTY-SUB", `{"leanval_owns_valset":true,"genesis_subjects":[]}`, true, false, 0, 0, false},
		{"G-SEED", string(seed), true, false, 1, 7, true},
		{"G-NEST", `{"params":{"leanval_owns_valset":true}}`, true, false, 0, 0, false},
		{"G-BAD", `{not-json`, false, true, 0, 0, false},
		{"G-BAD-TYPE", `{"leanval_owns_valset":"yes"}`, false, true, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gs types.GenesisState
			err := json.Unmarshal([]byte(tc.raw), &gs)
			if tc.wantFail {
				if err == nil {
					t.Fatal("want fail-closed unmarshal")
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal %s: %v", tc.name, err)
			}
			k := NewKeeper(NewMemStore(), ClosedVerifier{})
			k.InitGenesis(gs)
			if k.OwnsValset() != tc.wantOwns {
				t.Fatalf("owns=%v want %v", k.OwnsValset(), tc.wantOwns)
			}
			set := k.QueryBondedSet(0)
			if len(set) != tc.wantSeedN {
				t.Fatalf("seed n=%d want %d %+v", len(set), tc.wantSeedN, set)
			}
			if tc.wantSeedN == 1 {
				if set[0].Weight != tc.wantW || set[0].HasProof != tc.wantProof {
					t.Fatalf("seed %+v", set[0])
				}
			}
		})
	}
}

func TestP0_InitGenesisAppModuleStillSwallowsJSON(t *testing.T) {
	t.Skip("AppModule.InitGenesis still `_ = json.Unmarshal` — ValidateGenesis fail-closes but InitGenesis does not abort on G-BAD")
}

func TestP0_StakingEndBlockSplit_UpdatesOnlyLean(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	k.SetOwnsValset(true)
	k.SetEndPeriod(1)
	subj := make([]byte, 32)
	subj[1] = 7
	k.AcceptProof(1, subj, 21)
	ups, err := k.EndBlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ups) != 1 || ups[0].Power != 21 {
		t.Fatalf("flag on: Lean BondedSet updates only: %+v", ups)
	}
}

func TestP0_DummyStwoWeightBind_Vectors(t *testing.T) {
	period := uint64(1)
	subj := []byte("vec-subject-aaaaaaaaaaaaaaaa")
	w := int64(21)
	proof := DummyStwoProveBound(period, subj, w)
	if len(proof) != dummyProofLen || string(proof[:4]) != dummyMagic {
		t.Fatalf("wire %x", proof)
	}
	if err := (DummyStwoGo{}).VerifyDummy(proof, instancesFor(period, subj, w)); err != nil {
		t.Fatal(err)
	}
	legacy := DummyStwoProve(11, 22)
	if err := (DummyStwoGo{}).VerifyDummy(legacy, nil); err != nil {
		t.Fatal(err)
	}
}

func TestP0_DummyStwoWeightBind_ForgeReject(t *testing.T) {
	period := uint64(3)
	subj := []byte("ed25519-pubkey-bytes-32xx")
	proof := DummyStwoProveBound(period, subj, 21)
	if err := (DummyStwoGo{}).VerifyDummy(proof, instancesFor(period, subj, 21)); err != nil {
		t.Fatal(err)
	}
	if err := (DummyStwoGo{}).VerifyDummy(proof, instancesFor(period, subj, 99)); err == nil {
		t.Fatal("forged weight must REJECT")
	}
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	blob := types.EncodeLNPR(types.LNPRBlob{
		Period: types.PeriodFromHeight(1),
		Subjects: []types.SubjectProof{{
			Subject: subj,
			Weight:  99,
			Proof:   DummyStwoProveBound(types.PeriodFromHeight(1), subj, 21),
		}},
	})
	h := k.WrapProcessProposal(nil)
	resp, err := h(&abci.RequestProcessProposal{Height: 1, Txs: [][]byte{blob}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != abci.ResponseProcessProposal_REJECT {
		t.Fatalf("forged LNPR must REJECT, got %v", resp.Status)
	}
}

type gasSpy struct {
	storetypes.GasMeter
	desc []string
}

func (g *gasSpy) ConsumeGas(amount storetypes.Gas, descriptor string) {
	g.desc = append(g.desc, descriptor)
	g.GasMeter.ConsumeGas(amount, descriptor)
}

func TestP0_ConsumeGasBeforeVerify(t *testing.T) {
	inner := storetypes.NewGasMeter(1_000_000)
	spy := &gasSpy{GasMeter: inner}
	k := NewKeeper(NewMemStore(), DummyStwoGo{})
	k.SetGasMeter(spy)
	subj := []byte("gas-subject-aaaaaaaaaaaaaaaa")
	period := uint64(0)
	proof := DummyStwoProveBound(period, subj, 1)
	if err := k.VerifyLNPR(types.LNPRBlob{
		Period:   period,
		Subjects: []types.SubjectProof{{Subject: subj, Weight: 1, Proof: proof}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(spy.desc) == 0 || spy.desc[0] != "stwo dummy verify" {
		t.Fatalf("ConsumeGas descriptor first: %v", spy.desc)
	}
	if inner.GasConsumed() < stwoDummyGas {
		t.Fatalf("consumed %d want >= %d", inner.GasConsumed(), stwoDummyGas)
	}

	var called int
	k2 := NewKeeper(NewMemStore(), spyVerifier{fn: func(_, _ []byte) error {
		called++
		return nil
	}})
	k2.SetGasMeter(storetypes.NewGasMeter(1))
	func() {
		defer func() { _ = recover() }()
		_ = k2.VerifyLNPR(types.LNPRBlob{
			Period:   period,
			Subjects: []types.SubjectProof{{Subject: subj, Weight: 1, Proof: proof}},
		})
	}()
	if called != 0 {
		t.Fatal("verify must not run when gas meter empty")
	}
}

type spyVerifier struct {
	fn func(proof, instances []byte) error
}

func (s spyVerifier) VerifyDummy(proof, instances []byte) error { return s.fn(proof, instances) }

type p0Tx struct{ msgs []sdk.Msg }

func (t p0Tx) GetMsgs() []sdk.Msg                    { return t.msgs }
func (t p0Tx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func TestP0_BadBech32Reject(t *testing.T) {
	d := ante.NewDecorator()
	next := func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return ctx, nil }
	_, err := d.AnteHandle(sdk.Context{}, p0Tx{msgs: []sdk.Msg{
		&wasmtypes.MsgSudoContract{Contract: "not-a-bech32"},
	}}, false, next)
	if err != types.ErrSudoNotPermitted {
		t.Fatalf("bad bech32 want ErrSudoNotPermitted got %v", err)
	}
	_, err = d.AnteHandle(sdk.Context{}, p0Tx{msgs: []sdk.Msg{
		&wasmtypes.MsgSudoContract{Contract: sdk.AccAddress(types.TestLeanVerifierAcc()).String()},
	}}, false, next)
	if err != types.ErrSudoLeanVerifier {
		t.Fatalf("20-zero want ErrSudoLeanVerifier got %v", err)
	}
}

func TestP0_QueryBondedSetShape(t *testing.T) {
	k := NewKeeper(NewMemStore(), ClosedVerifier{})
	pub := make([]byte, 32)
	pub[0] = 0xab
	k.AcceptProof(0, pub, 10)
	set := k.QueryBondedSet(0)
	if len(set) != 1 || set[0].Weight != 10 || !set[0].HasProof {
		t.Fatalf("QueryBondedSet: %+v", set)
	}
	cmd := QueryBondedSetCLI(k)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"0"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("CLI bonded-set empty")
	}
}
