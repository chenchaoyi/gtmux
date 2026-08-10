package radar

import (
	"os"
	"strings"
	"testing"
)

// The BUG SURFACE: classifyStuck must not mark an idle Codex as holding a draft. Real
// Codex 0.146.0 idle capture (faint placeholder + truecolor status footer). Before the
// ansi (applyFaint truecolor) + dispatch (splitByPrompt footer) fixes this returned
// "draft" — an idle probe pane knocked on the commander as `waiting` for 8 hours.
func TestClassifyStuck_CodexIdle_NotDraft(t *testing.T) {
	b, err := os.ReadFile("testdata/codex-idle.ansi")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	colorCap := string(b)
	if got := classifyStuck(colorCap, "codex", true); got != "" {
		t.Fatalf("an idle Codex composer must classify as not-stuck; got %q", got)
	}
}

// Control: a genuine unsubmitted draft in a no-box composer must STILL be caught — the fix
// must not blind the detector. Real truecolor footer, a plain (non-faint) draft on the prompt.
func TestClassifyStuck_CodexWithDraft_IsDraft(t *testing.T) {
	colorCap := strings.Join([]string{
		"transcript above",
		"\x1b[1m❯\x1b[0m REAL_UNSUBMITTED_DRAFT here",
		"",
		"  \x1b[38;2;246;226;183mgpt-5.6-sol high\x1b[39m · \x1b[38;2;171;223;167m/private/tmp\x1b[39m",
	}, "\n")
	if got := classifyStuck(colorCap, "codex", true); got != "draft" {
		t.Fatalf("a real unsubmitted draft must classify as draft; got %q", got)
	}
}
