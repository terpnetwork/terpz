//go:build !cgo || nolink_libwasmvm

package keeper

import "fmt"

func verifyStwoInProcess(proof, instances []byte) error {
	_ = proof
	_ = instances
	return fmt.Errorf("leanval: wasmvm Stwo host not linked")
}
