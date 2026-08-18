package keeper

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("LEANVAL_MEMBERSHIP_DISABLE", "1")
	os.Exit(m.Run())
}
