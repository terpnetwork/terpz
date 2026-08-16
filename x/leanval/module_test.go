package leanval_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/terpnetwork/terp-core/v6/x/leanval"
	"github.com/terpnetwork/terp-core/v6/x/leanval/keeper"
)

func TestValidateGenesisFailClosed(t *testing.T) {
	var b leanval.AppModuleBasic
	if err := b.ValidateGenesis(nil, nil, []byte(`{not-json`)); err == nil {
		t.Fatal("bad JSON must fail ValidateGenesis")
	}
}

func TestValidateGenesisOwnsEmptyFails(t *testing.T) {
	var b leanval.AppModuleBasic
	if err := b.ValidateGenesis(nil, nil, []byte(`{"leanval_owns_valset":true}`)); err == nil {
		t.Fatal("owns without subjects must fail")
	}
	if err := b.ValidateGenesis(nil, nil, []byte(`{"leanval_owns_valset":true,"genesis_subjects":[]}`)); err == nil {
		t.Fatal("owns empty subjects must fail")
	}
}

func TestValidateGenesisOK(t *testing.T) {
	var b leanval.AppModuleBasic
	if err := b.ValidateGenesis(nil, nil, []byte(`{"leanval_owns_valset":false}`)); err != nil {
		t.Fatal(err)
	}
	ok := `{"leanval_owns_valset":true,"genesis_subjects":[{"pubkey":"AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=","weight":10}]}`
	if err := b.ValidateGenesis(nil, nil, []byte(ok)); err != nil {
		t.Fatal(err)
	}
}

func TestInitGenesisPanicsBadJSON(t *testing.T) {
	am := leanval.NewAppModule(keeper.NewKeeper(keeper.NewMemStore(), keeper.ClosedVerifier{}))
	defer func() {
		if recover() == nil {
			t.Fatal("InitGenesis must panic on G-BAD JSON")
		}
	}()
	am.InitGenesis(sdk.Context{}, nil, []byte(`{not-json`))
}
