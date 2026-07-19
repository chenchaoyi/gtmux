package prompt

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseOptions_ClaudeBox(t *testing.T) {
	text := `╭───────────────────────────────────────────╮
│ Do you want to make this edit to serve.go?  │
│                                             │
│ ❯ 1. Yes                                    │
│   2. Yes, and don't ask again this session  │
│   3. No, and tell Claude what to do (esc)   │
╰───────────────────────────────────────────╯`
	got := ParseOptions(text)
	want := []Option{
		{1, "Yes"},
		{2, "Yes, and don't ask again this session"},
		{3, "No, and tell Claude what to do (esc)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_PlainAndSelectorVariants(t *testing.T) {
	text := "Continue?\n  1. Yes\n> 2. No"
	got := ParseOptions(text)
	want := []Option{{1, "Yes"}, {2, "No"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_LatestMenuWins(t *testing.T) {
	// An older menu scrolled up, a fresh one below — the fresh run (restart at 1)
	// must win, not a concatenation.
	text := "1. old-a\n2. old-b\n... lots of output ...\n1. new-a\n2. new-b\n3. new-c"
	got := ParseOptions(text)
	want := []Option{{1, "new-a"}, {2, "new-b"}, {3, "new-c"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_StripsANSIColors(t *testing.T) {
	// A colored capture (capture-pane -e) leaks SGR codes into the menu; the ESC
	// byte is non-printing so users saw the bare "[38;5;153m…[0m" in the option
	// labels (reported 2026-06-29). The parser must strip them.
	esc := "\x1b"
	text := "❯ 1. " + esc + "[38;5;153mgtmux update" + esc + "[39m 之后" + esc + "[1m 自动" + esc + "[0m\n" +
		"  2. " + esc + "[31mNo" + esc + "[0m, keep current"
	got := ParseOptions(text)
	want := []Option{
		{1, "gtmux update 之后 自动"},
		{2, "No, keep current"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_StripsOSCHyperlinks(t *testing.T) {
	// Claude Code wraps file paths in OSC 8 hyperlinks; strip them too.
	esc := "\x1b"
	bel := "\x07"
	text := "1. open " + esc + "]8;;file:///tmp/a.go" + bel + "a.go" + esc + "]8;;" + bel + " now"
	got := ParseOptions(text)
	want := []Option{{1, "open a.go now"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_None(t *testing.T) {
	if got := ParseOptions("just some logs\nno menu here\nworking…"); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestParseOptions_GapBreaksRun(t *testing.T) {
	// "3." without a preceding 2. must not attach to option 1.
	text := "1. only\nsome noise\n3. orphan"
	got := ParseOptions(text)
	want := []Option{{1, "only"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

// WaitingOptions must fire on a live approval menu at the bottom (selector + ≥2
// options) but NOT on a numbered list in prose output, nor a menu far from the
// bottom — that's what keeps screen-based waiting detection from false alarms.
func TestWaitingOptions(t *testing.T) {
	codexMenu := "› Allow Codex to run this command?\n\n  › 1. Yes\n    2. Yes, don't ask again\n    3. No, tell Codex what to do\n"
	if got := WaitingOptions(codexMenu); len(got) != 3 || got[0].Label != "Yes" {
		t.Errorf("codex approval menu → %#v, want 3 options", got)
	}

	// a numbered list in prose (no selector cursor) must NOT read as waiting
	proseList := "Here's the plan:\n1. First refactor the parser\n2. Then add tests\n3. Finally ship it\nRunning now…\n"
	if got := WaitingOptions(proseList); got != nil {
		t.Errorf("prose numbered list → %#v, want nil", got)
	}

	// a menu buried far above the bottom (agent moved on) must NOT read as waiting
	buried := "› 1. Yes\n  2. No\n" + strings.Repeat("output line\n", 20)
	if got := WaitingOptions(buried); got != nil {
		t.Errorf("menu far from bottom → %#v, want nil", got)
	}

	// a single "1." with a selector isn't enough (need ≥2 real choices)
	if got := WaitingOptions("› 1. Only one\n"); got != nil {
		t.Errorf("single option → %#v, want nil", got)
	}

	// Claude's session-startup RESUME picker is a numbered menu with a selector,
	// but it's pre-task chrome — an old session reopened to it must NOT read as
	// "needs you" (the "2h-old session stuck waiting" bug).
	resume := "  This session is 22h 38m old and 166.8k tokens.\n" +
		"  Resuming the full session will consume a substantial portion of your usage limits.\n" +
		"  ❯ 1. Resume from summary (recommended)\n" +
		"    2. Resume full session as-is\n" +
		"    3. Don't ask me again\n" +
		"  Enter to confirm · Esc to cancel\n"
	if got := WaitingOptions(resume); got != nil {
		t.Errorf("resume picker → %#v, want nil (startup chooser, not a task-wait)", got)
	}

	// the trust-folder gate is likewise a startup chooser, not a task approval
	trust := "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n"
	if got := WaitingOptions(trust); got != nil {
		t.Errorf("trust-folder gate → %#v, want nil", got)
	}
}

// IsStartupGate detects an agent's PRE-TURN blocking gate (trust-folder confirmation) so
// the radar reads a stuck-before-running dispatch as waiting — but NOT the resume/theme
// picker (a reopened session must never read waiting forever). Per-agent, default set.
func TestIsStartupGate(t *testing.T) {
	trust := "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n"
	if !IsStartupGate(trust, "") {
		t.Error("trust-folder gate should be a startup gate")
	}
	resume := "  ❯ 1. Resume from summary (recommended)\n    2. Resume full session as-is\n"
	if IsStartupGate(resume, "") {
		t.Error("resume picker is NOT a gate — a reopened session must not read waiting")
	}
	if IsStartupGate("normal idle prompt\n❯ ", "") {
		t.Error("a normal idle screen is not a gate")
	}
	// A named agent still checks the default gate set.
	if !IsStartupGate(trust, "Claude Code") {
		t.Error("a named agent should still match the default gate set")
	}
}

// IsComposerReady is the SCREEN half of the spawn readiness handshake: a pane is ready
// to take a pasted goal only when its input prompt row is present, no startup/trust
// gate is up, and no boot banner is still on screen.
func TestIsComposerReady(t *testing.T) {
	// A clean idle Claude composer is ready.
	if !IsComposerReady("some earlier output\n\n❯ ", "") {
		t.Error("a clean idle composer should be ready")
	}
	// A boot banner (MCP auth / connecting) is NOT ready — the composer hasn't taken over.
	banners := []string{
		"❯ \n\n2 MCP servers need authentication",
		"Connecting to MCP servers…\n❯ ",
		"Starting Claude Code…",
		"Loading…\n❯ ",
	}
	for _, b := range banners {
		if IsComposerReady(b, "") {
			t.Errorf("boot banner should NOT be ready: %q", b)
		}
	}
	// A trust/permission gate is NOT ready (pasting through it would eat the keypress).
	trust := "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n"
	if IsComposerReady(trust, "") {
		t.Error("trust gate should NOT be ready")
	}
	// A live approval menu is a CHOICE wait, not a goal-ready composer.
	menu := "  ❯ 1. Yes\n    2. No\n    3. Always\n"
	if IsComposerReady(menu, "") {
		t.Error("a live approval menu should NOT be ready (waiting for a choice, not a goal)")
	}
	// A blank / still-booting screen with no prompt row is NOT ready.
	if IsComposerReady("\n\n\n", "") {
		t.Error("a screen with no prompt row should NOT be ready")
	}
	// A named agent still checks the default banner set.
	if IsComposerReady("2 MCP servers need authentication\n❯ ", "Claude Code") {
		t.Error("a named agent should still match the default boot-banner set")
	}
}

// hasBootBanner matches the default set and a named agent's own phrases.
func TestHasBootBanner(t *testing.T) {
	if !hasBootBanner("all good\nMCP servers need authentication\n❯ ", "") {
		t.Error("MCP auth notice is a boot banner")
	}
	if hasBootBanner("normal idle\n❯ ", "") {
		t.Error("a clean idle screen has no boot banner")
	}
}
