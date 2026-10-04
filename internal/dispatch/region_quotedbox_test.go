package dispatch

import (
	"strings"
	"testing"
)

// A Codex screen (0.160.0) whose transcript shows a Claude input box — the tail of a pane
// HQ captured with `tmux capture-pane`, printed as tool output. The text is neutral; the
// shape is the one the HQ pane showed on 2026-10-03 when six wakes were dropped as
// unconfirmed although each had arrived.
func codexWithQuotedBox(rules int, below ...string) string {
	quoted := []string{
		"• Ran tmux capture-pane -p -t %5",
		"  └   open that PR?",
		"",
		"    ✻ Crunched for 16s · done 11:15 PM",
		"",
	}
	if rules == 2 {
		quoted = append(quoted, "    ──────────────────────────────────────────────── dev ─")
	}
	quoted = append(quoted,
		"    ❯ a quoted draft in someone else's input box",
		"    ────────────────────────────────────────────────────────",
		"      ⏵⏵ auto mode on (shift+tab to cycle)",
		"",
		"  Worked for <1s • 14:34",
		"",
	)
	return strings.Join(append(quoted, below...), "\n")
}

var codexFooter = []string{"", "  GPT-6.1-Sol high · ~/work", "  ← for agents · ? for shortcuts"}

func TestQuotedBoxIsTranscriptNotTheInputBox(t *testing.T) {
	queued := append([]string{
		"• Messages to be submitted after next tool call (press esc to interrupt and send",
		"  immediately)",
		"  ↳ » ◆ gtmux·goal-changed dev:0.0 (%5) │ goal:\"probe\" · #f8fcff",
		"",
		"› Ask Codex to do anything",
	}, codexFooter...)
	submitted := append([]string{
		"› » ◆ gtmux·goal-changed dev:0.0 (%5) │ goal:\"probe\" · #f8fcff",
		"",
		"• Working (1s • esc to interrupt)",
		"",
		"› Ask Codex to do anything",
	}, codexFooter...)
	pasted := append([]string{
		"• Working (1s • esc to interrupt)",
		"",
		"› » ◆ gtmux·goal-changed dev:0.0 (%5) │ goal:\"probe\" · #f8fcff",
	}, codexFooter...)
	for _, tc := range []struct {
		name            string
		screen          string
		inHist, inDraft bool
	}{
		{"queued behind a tool call, two-rule quoted box", codexWithQuotedBox(2, queued...), true, false},
		{"queued behind a tool call, one-rule quoted box", codexWithQuotedBox(1, queued...), true, false},
		{"submitted", codexWithQuotedBox(2, submitted...), true, false},
		{"pasted, not submitted: still the composer", codexWithQuotedBox(2, pasted...), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			history, draft, structured := SplitInputRegion(tc.screen)
			if !structured {
				t.Fatal("the composer is a structure")
			}
			if strings.Contains(history, "#f8fcff") != tc.inHist || strings.Contains(draft, "#f8fcff") != tc.inDraft {
				t.Fatalf("id in history=%v draft=%v, want %v/%v\ndraft: %q",
					strings.Contains(history, "#f8fcff"), strings.Contains(draft, "#f8fcff"), tc.inHist, tc.inDraft, draft)
			}
			if strings.Contains(draft, "quoted draft") {
				t.Fatalf("the quoted box was taken for the input: draft %q", draft)
			}
		})
	}
}

// The rule only fires with a prompt BELOW the last border. A real Claude input box keeps
// its footer below it, and that is not a prompt, so its draft stays the box's content.
func TestClaudeBoxWithFooterIsStillTheInputBox(t *testing.T) {
	screen := strings.Join([]string{
		"⏺ Done.",
		"",
		"────────────────────────────────────────────────────────",
		"❯ typed but not sent",
		"────────────────────────────────────────────────────────",
		"  ⏵⏵ auto mode on (shift+tab to cycle) · ⧉ canvas",
		"  ? for shortcuts",
	}, "\n")
	_, draft, structured := SplitInputRegion(screen)
	if !structured || !strings.Contains(draft, "typed but not sent") {
		t.Fatalf("structured=%v draft=%q", structured, draft)
	}
}
