package leanval_test

import (
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval"
)

func TestValidateGenesisFailClosed(t *testing.T) {
	var b leanval.AppModuleBasic
	err := b.ValidateGenesis(nil, nil, []byte(`{not-json`))
	if err == nil {
		t.Fatal("bad JSON must fail ValidateGenesis")
	}
}

func TestValidateGenesisOK(t *testing.T) {
	var b leanval.AppModuleBasic
	if err := b.ValidateGenesis(nil, nil, []byte(`{"leanval_owns_valset":true}`)); err != nil {
		t.Fatal(err)
	}
}
