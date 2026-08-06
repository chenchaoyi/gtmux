package dispatch

import (
	"os"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/ansi"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// A REAL Codex 0.146.0 `capture-pane -e` of an IDLE composer (testdata/codex-idle.ansi):
// a faint placeholder ("Implement {feature}") on the prompt line, and BELOW it a truecolor
// status footer (model · cwd) separated by a blank row. This is the frame that made an idle
// Codex read as holding an unsubmitted draft for 8h. Two bugs compounded: (A) the truecolor
// footer's `38;2;r;g;b` was misread as SGR-2 faint, swallowing newlines and erasing the
// blank separator; (B) splitByPrompt then took the whole tail (footer included) as the draft.
func TestDraftOfColored_CodexIdlePlaceholder_NotADraft(t *testing.T) {
	draft, structured := DraftOfColored(readFixture(t, "codex-idle.ansi"))
	if strings.TrimSpace(draft) != "" {
		t.Fatalf("an idle Codex (faint placeholder + truecolor footer) must read as NO draft; got structured=%v draft=%q", structured, draft)
	}
}

// Bug A in isolation: a truecolor footer must survive the faint-drop, and its newline must
// too; a genuine SGR-2 faint span must still be dropped.
func TestStripDroppingFaint_TruecolorSurvives_RealFaintDropped(t *testing.T) {
	in := "\x1b[38;2;246;226;183mgpt-5.6-sol high\x1b[39m\n\x1b[38;2;171;223;167m/private/tmp\x1b[39m"
	out := ansi.StripDroppingFaint(in)
	if !strings.Contains(out, "gpt-5.6-sol high") || !strings.Contains(out, "/private/tmp") {
		t.Fatalf("truecolor text must survive faint-drop; got %q", out)
	}
	if !strings.Contains(out, "\n") {
		t.Fatalf("the newline between two truecolor lines must survive (was swallowed); got %q", out)
	}
	if got := ansi.StripDroppingFaint("\x1b[2mghost\x1b[0mreal"); got != "real" {
		t.Fatalf("a real SGR-2 faint span must still be dropped; got %q", got)
	}
}

// Bug B in isolation (no-box composer): the blank-separated status footer is NOT the draft;
// a CONTIGUOUS multi-line draft IS (so Deliver's multi-line verify does not regress).
func TestSplitByPrompt_FooterExcluded_ContiguousDraftKept(t *testing.T) {
	frame := strings.Join([]string{
		"transcript above",
		"❯ line one of a real draft",
		"  line two contiguous",
		"",
		"  gpt-5.6-sol high · /private/tmp", // footer, blank-separated
	}, "\n")
	_, draft, structured := SplitInputRegion(frame)
	if !structured {
		t.Fatal("a prompt line makes the region structured")
	}
	if !strings.Contains(draft, "line one of a real draft") || !strings.Contains(draft, "line two contiguous") {
		t.Fatalf("a contiguous multi-line draft must survive; got %q", draft)
	}
	if strings.Contains(draft, "gpt-5.6-sol") || strings.Contains(draft, "/private/tmp") {
		t.Fatalf("the blank-separated status footer must NOT be in the draft; got %q", draft)
	}
}

// applyFaint must treat extended-color sub-parameters as color, not intensity.
func TestApplyFaint_ExtendedColorNotIntensity(t *testing.T) {
	for _, in := range []string{
		"\x1b[48;2;50;50;53mBG\x1b[0m",     // truecolor bg (real from the codex frame)
		"\x1b[38;2;171;223;167mFG\x1b[39m", // truecolor fg
		"\x1b[38;5;2mIDX\x1b[39m",          // 256-color index 2 — tripped the old code
		"\x1b[38;5;22mIDX\x1b[39m",         // 256-color index 22
	} {
		if got := ansi.StripDroppingFaint(in); !strings.ContainsAny(got, "BFI") {
			t.Fatalf("extended-color text must survive (not dropped as faint): in=%q got=%q", in, got)
		}
	}
}
