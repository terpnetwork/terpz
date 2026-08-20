package types

import "testing"

func TestPeriodFromHeightHonorsEnv(t *testing.T) {
	t.Setenv("LEAN_BLOCKS_PER_PERIOD", "8")
	if BlocksPerPeriod != 600 {
		t.Fatalf("const must stay 600, got %d", BlocksPerPeriod)
	}
	if BlocksPerPeriodLive() != 8 {
		t.Fatalf("live %d", BlocksPerPeriodLive())
	}
	if PeriodFromHeight(7) != 0 || PeriodFromHeight(8) != 1 {
		t.Fatalf("7 -> %d 8 -> %d", PeriodFromHeight(7), PeriodFromHeight(8))
	}
}

func TestPeriodFromHeightDefault600(t *testing.T) {
	t.Setenv("LEAN_BLOCKS_PER_PERIOD", "")
	if PeriodFromHeight(599) != 0 || PeriodFromHeight(600) != 1 {
		t.Fatal("default period = height / 600")
	}
}
