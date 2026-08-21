package types

import "testing"

func TestPeriodFromHeightHonorsGenesisParam(t *testing.T) {
	t.Cleanup(func() { SetBlocksPerPeriod(0) })
	if BlocksPerPeriod != 600 {
		t.Fatalf("const must stay 600, got %d", BlocksPerPeriod)
	}
	SetBlocksPerPeriod(8)
	if BlocksPerPeriodLive() != 8 {
		t.Fatalf("live %d", BlocksPerPeriodLive())
	}
	if PeriodFromHeight(7) != 0 || PeriodFromHeight(8) != 1 {
		t.Fatalf("7 -> %d 8 -> %d", PeriodFromHeight(7), PeriodFromHeight(8))
	}
}

func TestPeriodFromHeightDefault600(t *testing.T) {
	t.Cleanup(func() { SetBlocksPerPeriod(0) })
	SetBlocksPerPeriod(0)
	if PeriodFromHeight(599) != 0 || PeriodFromHeight(600) != 1 {
		t.Fatal("default period = height / 600")
	}
}

func TestPeriodFromHeightIgnoresEnv(t *testing.T) {
	t.Cleanup(func() { SetBlocksPerPeriod(0) })
	t.Setenv("LEAN_BLOCKS_PER_PERIOD", "8")
	SetBlocksPerPeriod(0)
	if PeriodFromHeight(8) != 0 {
		t.Fatalf("env must not fork period, got %d", PeriodFromHeight(8))
	}
	if PeriodFromHeight(600) != 1 {
		t.Fatal("genesis default 600")
	}
}
