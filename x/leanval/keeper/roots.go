package keeper

import "github.com/terpnetwork/terp-core/v6/x/leanval/types"

// LastObjectRoots is the committed deposit||bitfield||EB tuple (96 bytes, zero default).
// VerifyLNPR binds Dummy instances to this value — vote-sdk store-sourced PIs.
func (k *Keeper) LastObjectRoots() []byte {
	out := make([]byte, types.ObjectRootsSize)
	if k.store == nil {
		return out
	}
	if b := k.live().Get(types.ObjectRootsKey()); len(b) > 0 {
		copy(out, b)
	}
	return out
}

// SetLastObjectRoots writes the 96-byte commitment (pad/truncate). Not a Stwo statement.
func (k *Keeper) SetLastObjectRoots(roots []byte) {
	out := make([]byte, types.ObjectRootsSize)
	copy(out, roots)
	k.live().Set(types.ObjectRootsKey(), out)
}
