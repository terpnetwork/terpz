//go:build cgo && !nolink_libwasmvm

package keeper

import wasmvm "github.com/CosmWasm/wasmvm/v3"

func verifyStwoInProcess(proof, instances []byte) error {
	return wasmvm.VerifyStwoHost(proof, instances)
}
