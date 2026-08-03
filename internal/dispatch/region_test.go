package dispatch

import (
	"strings"
	"testing"
)

func TestSplitInputRegion_Box(t *testing.T) {
	cap := strings.Join([]string{
		"user: earlier message",
		"assistant: an earlier reply",
		"╭──────────────────────────────╮",
		"│ ❯ my draft text here          │",
		"╰──────────────────────────────╯",
	}, "\n")
	history, draft, structured := SplitInputRegion(cap)
	if !structured {
		t.Fatalf("a box input must be structured")
	}
	if draft != "my draft text here" {
		t.Fatalf("draft = %q", draft)
	}
	if !strings.Contains(history, "earlier reply") || strings.Contains(history, "my draft text") {
		t.Fatalf("history leaked the draft or lost content: %q", history)
	}
}

func TestSplitInputRegion_EmptyDraftBox(t *testing.T) {
	cap := strings.Join([]string{
		"conversation line",
		"╭──────────────────────────────╮",
		"│ ❯                             │",
		"╰──────────────────────────────╯",
	}, "\n")
	history, draft, structured := SplitInputRegion(cap)
	if !structured {
		t.Fatalf("a box input must be structured")
	}
	if draft != "" {
		t.Fatalf("empty draft box should yield empty draft, got %q", draft)
	}
	if !strings.Contains(history, "conversation line") {
		t.Fatalf("history missing: %q", history)
	}
}

func TestSplitInputRegion_NoBoxDegradesToPrompt(t *testing.T) {
	cap := "line one\nline two\n❯ typed text"
	history, draft, structured := SplitInputRegion(cap)
	if !structured {
		t.Fatalf("a ❯ prompt must be structured")
	}
	if draft != "typed text" {
		t.Fatalf("no-box draft = %q", draft)
	}
	if !strings.Contains(history, "line two") {
		t.Fatalf("history = %q", history)
	}
}

func TestSplitInputRegion_PlainShellIsUnstructured(t *testing.T) {
	// A bare shell prompt has no locatable input region → structured=false, so the
	// deliver guard won't wipe the pasted text as a "fragment".
	cap := "user@host project % echo hello world"
	_, draft, structured := SplitInputRegion(cap)
	if structured {
		t.Fatalf("a plain shell prompt should be unstructured")
	}
	if draft != "" {
		t.Fatalf("unstructured draft should be empty, got %q", draft)
	}
}

func TestIsBoxBorder(t *testing.T) {
	for _, ok := range []string{"╭──────╮", "╰──────╯", "──────────", "├────┤"} {
		if !isBoxBorder(ok) {
			t.Fatalf("should be a border: %q", ok)
		}
	}
	for _, no := range []string{"│ ❯ hello │", "just text", "│", "─"} {
		if isBoxBorder(no) {
			t.Fatalf("should NOT be a border: %q", no)
		}
	}
}

// esc is the ANSI escape byte, kept out of the raw string literals below.
const esc = "\x1b"

func TestDraftOfColored_ExcludesFaintGhost(t *testing.T) {
	// A composer whose ONLY draft content is CC's faint suggested-next-command ghost text
	// (the reproduced bug: %85 showed `ESC[2m ping %14 … ESC[0m`). It must read EMPTY.
	ghost := strings.Join([]string{
		"assistant: an earlier reply",
		"╭──────────────────────────────╮",
		"│ ❯ " + esc + "[2mping %14 that the charter needs coordinating" + esc + "[0m │",
		"╰──────────────────────────────╯",
	}, "\n")
	draft, structured := DraftOfColored(ghost)
	if !structured {
		t.Fatalf("borders survive faint-strip → still structured")
	}
	if strings.TrimSpace(draft) != "" {
		t.Fatalf("a faint ghost suggestion must read as an EMPTY draft; got %q", draft)
	}

	// A real, bright user draft must still be detected.
	real := strings.Join([]string{
		"╭──────────────────────────────╮",
		"│ ❯ actually typed by the user  │",
		"╰──────────────────────────────╯",
	}, "\n")
	draft, structured = DraftOfColored(real)
	if !structured || strings.TrimSpace(draft) != "actually typed by the user" {
		t.Fatalf("a bright draft must survive; structured=%v draft=%q", structured, draft)
	}

	// Bright input with a faint autosuggestion tail keeps only the bright part.
	mixed := strings.Join([]string{
		"╭──────────────────────────────╮",
		"│ ❯ git com" + esc + "[2mmit -m done" + esc + "[0m │",
		"╰──────────────────────────────╯",
	}, "\n")
	draft, _ = DraftOfColored(mixed)
	if strings.TrimSpace(draft) != "git com" {
		t.Fatalf("mixed draft must keep only the bright input; got %q", draft)
	}
}

// opencode's composer is a LEFT-EDGE box: "┃  text" lines closed by a "╹▀▀▀"
// bottom rule with NO top rule — so the draft sits ABOVE the border, and the
// border uses half-block runes. Both were invisible to the region detector
// (task ②: gtmux send to opencode couldn't confirm the paste → flaky delivery).
func TestSplitInputRegion_OpencodeLeftEdgeBox(t *testing.T) {
	cap := strings.Join([]string{
		"  history line one",
		"  history line two",
		"  ┃",
		"  ┃  OPENCODE_DRAFT_xyz",
		"  ┃",
		"  ┃  Build · GPT-5 Mini OpenAI",
		"  ╹▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀",
		"   /private/tmp        ctrl+p commands",
	}, "\n")
	history, draft, structured := SplitInputRegion(cap)
	if !structured {
		t.Fatal("opencode composer must be structured (it has a ╹▀▀▀ bottom rule)")
	}
	if !strings.Contains(draft, "OPENCODE_DRAFT_xyz") {
		t.Errorf("draft = %q; must contain the pasted text from the ┃ block above the border", draft)
	}
	if strings.Contains(draft, "history line") {
		t.Errorf("draft leaked history: %q", draft)
	}
	if strings.Contains(draft, "ctrl+p commands") {
		t.Errorf("draft wrongly took the below-border status line: %q", draft)
	}
	if !strings.Contains(history, "history line two") {
		t.Errorf("history = %q; must include the lines above the box", history)
	}
}

func TestIsBoxBorder_OpencodeRuleNotBanner(t *testing.T) {
	if !isBoxBorder("╹▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀") {
		t.Error("opencode's ╹▀▀▀ bottom rule must count as a box border")
	}
	if !isBoxBorder("▀▀▀▀▀▀▀▀▀▀") {
		t.Error("a bare half-block rule must count as a box border")
	}
	// the opencode / ASCII banners are made of the FULL block █ — must NOT be borders,
	// or history would be mis-split.
	if isBoxBorder("█▀▀█ █▀▀█ █▀▀█ █▀▀▄") {
		t.Error("a full-block █ banner line must NOT be a box border")
	}
}

// A lone TOP rule (no left-edge box below its own line) keeps the old behavior:
// the draft is BELOW the rule.
func TestSplitInputRegion_TopRuleDraftBelowUnchanged(t *testing.T) {
	cap := strings.Join([]string{
		"history above",
		"────────────────────",
		"❯ draft below the rule",
	}, "\n")
	_, draft, structured := SplitInputRegion(cap)
	if !structured || !strings.Contains(draft, "draft below the rule") {
		t.Fatalf("top-rule case regressed: structured=%v draft=%q", structured, draft)
	}
}
