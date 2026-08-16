package keeper

import (
	"github.com/cosmos/cosmos-sdk/store/v2/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// KV wraps a bound SDK KVStore.
type KV struct{ S storetypes.KVStore }

func (k KV) Get(key []byte) []byte {
	if k.S == nil {
		return nil
	}
	return k.S.Get(key)
}
func (k KV) Set(key, value []byte) {
	if k.S != nil {
		k.S.Set(key, value)
	}
}
func (k KV) Delete(key []byte) {
	if k.S != nil {
		k.S.Delete(key)
	}
}
func (k KV) IteratePrefix(p []byte, fn func(key, value []byte) bool) {
	if k.S == nil {
		return
	}
	ps := prefix.NewStore(k.S, p)
	it := ps.Iterator(nil, nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		full := append(append([]byte{}, p...), it.Key()...)
		if !fn(full, it.Value()) {
			return
		}
	}
}

func BindKV(ctx sdk.Context, key storetypes.StoreKey) Store {
	return KV{S: ctx.KVStore(key)}
}
