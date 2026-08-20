package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// WasmSudoClient is zk-wasmd Keeper.Sudo — module-owned contract only.
type WasmSudoClient interface {
	Sudo(ctx context.Context, contractAddress sdk.AccAddress, msg []byte) ([]byte, error)
}

// DefaultLeanZkID is the first Lean Stwo circuit pin (prover_id=2, curve_id=5).
const DefaultLeanZkID uint64 = 1

func (k *Keeper) SetWasmSudo(c WasmSudoClient) { k.wasmSudo = c }

func (k *Keeper) SetModuleVerifier(addr []byte, zkid uint64) {
	k.verifierAcc = append([]byte(nil), addr...)
	if zkid == 0 {
		zkid = DefaultLeanZkID
	}
	k.leanZkID = zkid
}

// verifyProof prefers the module-owned verifier contract (sudo →
// deps.api.proof_instance_verify(zkid, …) on zk-wasmvm Stwo arm).
// If no contract is bound, DummyStwoGo remains the lab fallback.
func (k *Keeper) verifyProof(proof, instances []byte) error {
	if k.wasmSudo != nil && len(k.verifierAcc) > 0 {
		return k.sudoProofInstanceVerify(proof, instances)
	}
	if k.Verifier == nil {
		return fmt.Errorf("leanval: no verifier")
	}
	return k.Verifier.VerifyDummy(proof, instances)
}

// SudoProofInstanceVerifyMsg is the JSON the cw-lean-verifier sudo handler
// must accept. The contract calls the host module instance API:
//
//	deps.api.proof_instance_verify(zkid, proof, instances)
type SudoProofInstanceVerifyMsg struct {
	ProofInstanceVerify struct {
		ZkID      uint64 `json:"zkid"`
		Proof     []byte `json:"proof"`
		Instances []byte `json:"instances"`
	} `json:"proof_instance_verify"`
}

func (k *Keeper) sudoProofInstanceVerify(proof, instances []byte) error {
	if !k.hasCtx {
		return fmt.Errorf("leanval: wasm sudo requires BindContext")
	}
	zkid := k.leanZkID
	if zkid == 0 {
		zkid = DefaultLeanZkID
	}
	var msg SudoProofInstanceVerifyMsg
	msg.ProofInstanceVerify.ZkID = zkid
	msg.ProofInstanceVerify.Proof = proof
	msg.ProofInstanceVerify.Instances = instances
	bz, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = k.wasmSudo.Sudo(k.sdkCtx, sdk.AccAddress(k.verifierAcc), bz)
	if err != nil {
		return fmt.Errorf("leanval: module proof_instance_verify zkid=%d: %w", zkid, err)
	}
	return nil
}
