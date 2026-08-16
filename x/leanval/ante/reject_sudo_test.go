package ante_test

import (
	"testing"

	"github.com/terpnetwork/terp-core/v6/x/leanval/ante"
	"github.com/terpnetwork/terp-core/v6/x/leanval/types"
)

func TestRejectLeanVerifier(t *testing.T) {
	if err := ante.RejectLeanSudo(types.TestLeanVerifierAcc()); err != types.ErrSudoLeanVerifier {
		t.Fatalf("want ErrSudoLeanVerifier code=%d got %v", types.CodeSudoLeanVerifier, err)
	}
}

func TestAllowOther(t *testing.T) {
	other := make([]byte, 20)
	other[19] = 1
	if err := ante.RejectLeanSudo(other); err != nil {
		t.Fatal(err)
	}
}
