package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// The tree as it was found on the machine that exposed it: a retired spool next to two
// live cadence markers, icon caches at the data root, everything world-readable.
func TestHousekeepRetiresWhatNothingReadsAndKeepsWhatDoctorDoes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	base, cfg := state.Dir(), state.ConfigDir()
	write := func(p string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), mode); err != nil {
			t.Fatal(err)
		}
		_ = os.Chmod(p, mode)
		_ = os.Chmod(filepath.Dir(p), 0o755)
	}
	for _, rel := range retiredFiles {
		write(filepath.Join(base, rel), 0o644)
	}
	live := []string{filepath.Join(base, "hq-feed", "last-distill"), filepath.Join(base, "hq-feed", "last-self-check")}
	for _, p := range live {
		write(p, 0o644)
	}
	write(filepath.Join(base, "agent-icons", "claude.png"), 0o644)
	write(filepath.Join(base, "events.jsonl"), 0o644)
	write(filepath.Join(cfg, "hq", "notes", "board.md"), 0o644)
	_ = os.MkdirAll(filepath.Join(base, "briefs"), 0o755)

	Housekeep()

	for _, rel := range retiredFiles {
		if _, err := os.Stat(filepath.Join(base, rel)); !os.IsNotExist(err) {
			t.Errorf("retired %s survived", rel)
		}
	}
	for _, p := range live {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s is a live cadence marker doctor reads, and it was removed", filepath.Base(p))
		}
	}
	if _, err := os.Stat(filepath.Join(base, "agent-icons")); !os.IsNotExist(err) {
		t.Error("the icon cache at the data root was not retired (it lives under cache/ now)")
	}
	if _, err := os.Stat(filepath.Join(base, "briefs")); !os.IsNotExist(err) {
		t.Error("the empty briefs/ was not removed")
	}
	for _, p := range []string{filepath.Join(base, "events.jsonl"), filepath.Join(cfg, "hq", "notes", "board.md")} {
		if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
			t.Errorf("%s is %#o after housekeeping, want 0600", filepath.Base(p), fi.Mode().Perm())
		}
	}

	day := time.Now().Format("2006-01-02")
	b, _ := os.ReadFile(filepath.Join(state.LogsDir(), day+".jsonl"))
	log := string(b)
	for _, want := range []string{`"event":"act.narrow"`, `"target":"` + filepath.Join("hq-feed", "spool.jsonl") + `"`, `"target":"agent-icons"`} {
		if !strings.Contains(log, want) {
			t.Errorf("the log does not record %s:\n%s", want, log)
		}
	}
}
