package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestEffectiveResumeMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no config → default on

	// Flag overrides config.
	cases := []struct {
		flag string
		want resumeMode
	}{
		{"auto", resumeAuto},
		{"type", resumeType},
		{"off", resumeOff},
		{"", resumeAuto}, // no flag, no config → default auto
	}
	for _, c := range cases {
		restoreResumeFlag = c.flag
		if got := effectiveResumeMode(); got != c.want {
			t.Errorf("flag=%q → %v, want %v", c.flag, got, c.want)
		}
	}
	restoreResumeFlag = ""
}

func TestAutoResumeConfigToggle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".config", "gtmux")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	restoreResumeFlag = "" // follow config

	// Explicitly off → type (pre-fill, don't run) per the agreed UX.
	if err := os.WriteFile(filepath.Join(cfg, "config.json"),
		[]byte(`{"autoResumeAgentSessions": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := effectiveResumeMode(); got != resumeType {
		t.Errorf("config off → %v, want resumeType", got)
	}

	// Explicitly on → auto.
	os.WriteFile(filepath.Join(cfg, "config.json"), []byte(`{"autoResumeAgentSessions": true}`), 0o644)
	if got := effectiveResumeMode(); got != resumeAuto {
		t.Errorf("config on → %v, want resumeAuto", got)
	}
}

func TestIsShellCommand(t *testing.T) {
	for _, s := range []string{"bash", "zsh", "-zsh", "-bash", "fish", "sh"} {
		if !isShellCommand(s) {
			t.Errorf("isShellCommand(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"node", "claude", "codex", "python", "vim", ""} {
		if isShellCommand(s) {
			t.Errorf("isShellCommand(%q) = true, want false", s)
		}
	}
}

// hq-home-quarantine's sibling: the "multiple cc sessions after restore" bug. The
// old cwd-only fallback injected a historical conversation into any bare-shell pane
// that merely shared a project directory. pickCwdFallback now requires the record's
// original window.pane position to match, so a never-agent pane gets nothing.
func TestPickCwdFallback(t *testing.T) {
	proj := "/Users/x/proj/gtmux"
	all := []resume.Located{
		{Loc: "gtmux dev:0.0", Record: resume.Record{SessionID: "sess-00", Cwd: proj, UpdatedAt: 300}},
		{Loc: "old name:0.0", Record: resume.Record{SessionID: "sess-old00", Cwd: proj, UpdatedAt: 200}},
		{Loc: "gtmux dev:0.1", Record: resume.Record{SessionID: "sess-01", Cwd: proj, UpdatedAt: 100}},
	}
	used := map[string]bool{}
	noAge := time.Time{} // no dated save to compare against → age check off

	// A restored pane at window.pane 0.0 whose session was renamed: matches records at
	// position 0.0 (same dir). Newest wins (sess-00).
	rec, cands := pickCwdFallback("renamed:0.0", proj, all, used, noAge)
	if rec == nil || rec.SessionID != "sess-00" {
		t.Fatalf("position 0.0 should recover the newest 0.0 record; got %+v", rec)
	}
	if cands != 2 {
		t.Fatalf("two records at position 0.0 in this dir; got cands=%d", cands)
	}

	// THE BUG: a bare shell pane at position 0.9 that only shares the project dir —
	// no record ever lived at 0.9, so it must recover NOTHING (was: injected a
	// historical conversation).
	if rec, _ := pickCwdFallback("gtmux dev:0.9", proj, all, used, noAge); rec != nil {
		t.Fatalf("a pane at a position no agent ever used must not be injected; got %+v", rec)
	}

	// A different directory with no matching record → nothing.
	if rec, _ := pickCwdFallback("gtmux dev:0.0", "/other/dir", all, used, noAge); rec != nil {
		t.Fatalf("no record in this dir → no recovery; got %+v", rec)
	}

	// used dedup: once sess-00 is consumed, position 0.0 falls through to sess-old00.
	used["sess-00"] = true
	if rec, _ := pickCwdFallback("renamed:0.0", proj, all, used, noAge); rec == nil || rec.SessionID != "sess-old00" {
		t.Fatalf("a used record is skipped; got %+v", rec)
	}

	// empty cwd never matches (a pane with no dir can't be recovered by dir).
	if rec, _ := pickCwdFallback("x:0.0", "", all, used, noAge); rec != nil {
		t.Fatalf("empty cwd must not match; got %+v", rec)
	}
}

// The fallback is a GUESS — the record it finds belonged to a different locator. A
// record nobody has touched in weeks is far likelier to be one of the ghost locators
// that accumulate in the store (117 of them on the reporting machine) than the
// conversation that was live when the machine went down.
func TestPickCwdFallbackAgeBelt(t *testing.T) {
	proj := "/Users/x/proj/gtmux"
	saveTime := time.Date(2026, 8, 4, 7, 22, 0, 0, time.UTC)
	fresh := saveTime.Add(-2 * time.Hour).Unix()
	ancient := saveTime.Add(-30 * 24 * time.Hour).Unix()

	all := []resume.Located{
		{Loc: "ghost:0.0", Record: resume.Record{SessionID: "sess-ancient", Cwd: proj, UpdatedAt: ancient}},
		{Loc: "old name:0.0", Record: resume.Record{SessionID: "sess-fresh", Cwd: proj, UpdatedAt: fresh}},
	}
	// Newest-first ordering is the store's contract; the ancient one is only reachable
	// if the age belt lets it through.
	rec, cands := pickCwdFallback("renamed:0.0", proj, all[1:], map[string]bool{}, saveTime)
	if rec == nil || rec.SessionID != "sess-fresh" || cands != 1 {
		t.Fatalf("a recently-touched record must still be recovered; got %+v cands=%d", rec, cands)
	}
	if rec, cands := pickCwdFallback("renamed:0.0", proj, all[:1], map[string]bool{}, saveTime); rec != nil || cands != 0 {
		t.Fatalf("a month-old record must not be guessed into a pane; got %+v cands=%d", rec, cands)
	}
	// A record with no timestamp at all is not evidence of staleness — don't invent it.
	undated := []resume.Located{{Loc: "x:0.0", Record: resume.Record{SessionID: "sess-undated", Cwd: proj}}}
	if rec, _ := pickCwdFallback("renamed:0.0", proj, undated, map[string]bool{}, saveTime); rec == nil {
		t.Fatal("an undated record must not be dropped by the age belt")
	}
}

func TestPosSuffix(t *testing.T) {
	cases := map[string]string{
		"gtmux dev:0.1":      "0.1",
		"a:b:2.3":            "2.3", // colon in session name → split on LAST colon
		"你是worker(不是HQ):1.0": "1.0",
		"nocolon":            "nocolon",
	}
	for in, want := range cases {
		if got := posSuffix(in); got != want {
			t.Errorf("posSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}
