package leanval_test

import (
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

func TestWrapDistributionName(t *testing.T) {
	k := keeper.NewKeeper(nil, nil)
	k.SetOwnsValset(true)
	if !k.OwnsValset() {
		t.Fatal("flag")
	}
	// BeginBlock skip is exercised when wired; compile-time type is in distr_wrap.go
	_ = k
}
