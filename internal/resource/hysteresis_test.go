package resource

import (
	"os"
	"path/filepath"
	"testing"
)

func disk(freeGB int) Machine {
	return Machine{DiskFreeGB: freeGB, MemTier: "normal", NCPU: 8}
}

func load(ratio float64) Machine {
	return Machine{DiskFreeGB: 500, MemTier: "normal", LoadRatio: ratio, NCPU: 8}
}

// Defaults: disk red under 15 GB, clearing at 17 (15 + a 2 GB margin). The dither
// that was re-alerting — 15.1 → 14.9 → 15.1 — must hold red the whole way.
func TestMachineTierSticky_DiskHysteresis(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := MachineTierSticky(TierNormal, disk(14)); got != TierRed {
		t.Fatalf("under the red line, from normal: %v", got)
	}
	for _, freeGB := range []int{15, 16} { // above the entry line, inside the exit band
		if got := MachineTierSticky(TierRed, disk(freeGB)); got != TierRed {
			t.Fatalf("%d GB is inside red's exit band — the tier must hold, got %v", freeGB, got)
		}
	}
	if got := MachineTierSticky(TierRed, disk(17)); got != TierAmber {
		t.Fatalf("17 GB clears red (entry 15 + margin 2) and lands in amber; got %v", got)
	}
	// Rising is never damped: the entry threshold decides, so a real fall into red
	// is announced on the first sample that crosses it.
	if got := MachineTierSticky(TierAmber, disk(14)); got != TierRed {
		t.Fatalf("a rise uses the entry threshold; got %v", got)
	}
}

func TestMachineTierSticky_AmberHysteresis(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := MachineTierSticky(TierNormal, disk(49)); got != TierAmber {
		t.Fatalf("under the amber line: %v", got)
	}
	if got := MachineTierSticky(TierAmber, disk(51)); got != TierAmber {
		t.Fatalf("51 GB is inside amber's exit band (50 + 2) — hold; got %v", got)
	}
	if got := MachineTierSticky(TierAmber, disk(52)); got != TierNormal {
		t.Fatalf("52 GB clears amber; got %v", got)
	}
}

// Load ratio oscillating around 1.0× cores is the worst flapper: the amber line sits
// exactly where a busy machine's load lives.
func TestMachineTierSticky_LoadHysteresis(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := MachineTierSticky(TierNormal, load(1.05)); got != TierAmber {
		t.Fatalf("1.05× crosses the amber line; got %v", got)
	}
	for _, ratio := range []float64{0.95, 0.9, 0.86} { // inside the exit band (>= 0.85)
		if got := MachineTierSticky(TierAmber, load(ratio)); got != TierAmber {
			t.Fatalf("%.2f× is inside amber's exit band — hold; got %v", ratio, got)
		}
	}
	if got := MachineTierSticky(TierAmber, load(0.8)); got != TierNormal {
		t.Fatalf("0.8× clears amber (1.0 − 0.15); got %v", got)
	}
}

// Hysteresis governs the ALERT, never the readout: `gtmux resource` / the digest /
// GET /api/usage keep reporting raw truth (a display that jitters wakes nobody).
func TestSnapshotStaysRaw(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := disk(16)
	if got := m.WarnTier(loadConfig()); got != TierAmber {
		t.Fatalf("16 GB free is raw-amber (under 50, over 15); got %v", got)
	}
	if MachineTierSticky(TierRed, m) != TierRed {
		t.Fatal("…while the ALERT still holds red — the two are deliberately different")
	}
	if evalMachine(m, loadConfig()) != "disk 16GB free" {
		t.Fatalf("the reported warn string is the raw reading; got %q", evalMachine(m, loadConfig()))
	}
}

// The margins are configurable, and a config naming only ONE key still gets working
// defaults for the rest (an unset field reads as 0 → default).
func TestConfig_HysteresisKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"resource":{"diskRedGB":20,"diskHysteresisGB":5}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	c := loadConfig()
	if c.DiskRedGB != 20 || c.DiskHysteresisGB != 5 {
		t.Fatalf("configured keys must win; got red=%d hyst=%d", c.DiskRedGB, c.DiskHysteresisGB)
	}
	if c.ConfirmSamples != defaultConfig.ConfirmSamples || c.LoadHysteresis != defaultConfig.LoadHysteresis {
		t.Fatalf("unnamed keys take the default; got confirm=%d loadHyst=%v",
			c.ConfirmSamples, c.LoadHysteresis)
	}
	if got := MachineTierSticky(TierRed, disk(24)); got != TierRed {
		t.Fatalf("24 GB is inside the configured exit band (20 + 5); got %v", got)
	}
	if got := MachineTierSticky(TierRed, disk(25)); got != TierAmber {
		t.Fatalf("25 GB clears it; got %v", got)
	}
}
