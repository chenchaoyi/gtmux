package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeSaveWithAge creates a temp file whose mtime is `age` in the past, returning its path.
func writeSaveWithAge(t *testing.T, age time.Duration) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "last")
	if err := os.WriteFile(p, []byte("state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mt := time.Now().Add(-age)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSaveIsStale(t *testing.T) {
	now := time.Now()
	fresh := writeSaveWithAge(t, 2*time.Minute)
	old := writeSaveWithAge(t, 30*time.Minute)

	if saveIsStale(fresh, now, backstopSaveStaleAfter) {
		t.Errorf("a 2m-old save should NOT be stale at a %v threshold", backstopSaveStaleAfter)
	}
	if !saveIsStale(old, now, backstopSaveStaleAfter) {
		t.Errorf("a 30m-old save SHOULD be stale at a %v threshold", backstopSaveStaleAfter)
	}
	if !saveIsStale("", now, backstopSaveStaleAfter) {
		t.Error("an empty path (no save) should be treated as stale")
	}
	if !saveIsStale(filepath.Join(t.TempDir(), "nope"), now, backstopSaveStaleAfter) {
		t.Error("a missing file should be treated as stale")
	}
}

func TestSaveStalenessWarning(t *testing.T) {
	now := time.Now()
	if w := saveStalenessWarning(writeSaveWithAge(t, time.Hour), now); w != "" {
		t.Errorf("a 1h-old save should not warn, got %q", w)
	}
	if w := saveStalenessWarning(writeSaveWithAge(t, 18*24*time.Hour), now); w == "" {
		t.Error("an 18-day-old save should produce a staleness warning")
	}
	if w := saveStalenessWarning("", now); w != "" {
		t.Errorf("no save → no warning, got %q", w)
	}
}

func TestStatusRightHasContinuumTrigger(t *testing.T) {
	with := "#[fg=blue] #{b:pane_current_path} %H:%M #(~/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh)"
	without := "#[fg=blue] #{b:pane_current_path} %H:%M "
	if !statusRightHasContinuumTrigger(with) {
		t.Error("status-right carrying continuum_save.sh should be detected as armed")
	}
	if statusRightHasContinuumTrigger(without) {
		t.Error("a custom status-right without the trigger should be detected as disarmed")
	}
}
