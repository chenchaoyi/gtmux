package prompt

import (
	"strings"
	"testing"
)

// IsStartupPicker is the twin of IsStartupGate for REOPENED-SESSION chrome. The two are
// deliberately different verdicts: a gate blocks work and reads as needs-you, a picker is
// a menu a returning session sits at and must never read as waiting. What they share is
// the narrowing — both answer "is the agent DRAWING this right now?", not "does the word
// appear anywhere on screen?".
func TestIsStartupPicker(t *testing.T) {
	resume := "  ❯ 1. Resume from summary (recommended)\n    2. Resume full session as-is\n    3. Start fresh\n"
	if !IsStartupPicker(resume, "") {
		t.Error("Claude's resume menu should be recognized as a startup picker")
	}
	if !IsStartupPicker(resume, "Claude Code") {
		t.Error("a named agent must still match the default picker set — the registry lookup takes a display label too")
	}
	if IsStartupPicker("normal idle prompt\n❯ ", "") {
		t.Error("an idle composer is not a picker")
	}
	trust := "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n"
	if IsStartupPicker(trust, "") {
		t.Error("the trust gate is a GATE, not a picker — the two get different verdicts and must not blur")
	}
}

// The narrowing, in the shape it has bitten twice already (2026-08-09 gate phrases,
// 2026-08-10 startup wording): a pane that MENTIONS the menu's text — a worker reading
// this very table, say — is not sitting at the menu.
func TestIsStartupPicker_QuotedInScrollbackIsNotAPicker(t *testing.T) {
	quoted := "\t\t\"Resume from summary\",       // Claude resume picker\n"
	tail := strings.Repeat("  ok\n", gateRegionLines) + "❯ "
	if IsStartupPicker(quoted+tail, "") {
		t.Error("picker wording quoted up in scrollback must not read as a live picker")
	}
	// Still inside the bottom region → still a picker. The boundary belongs to the menu.
	near := quoted + strings.Repeat("  ok\n", gateRegionLines-4) + "❯ "
	if !IsStartupPicker(near, "") {
		t.Error("a picker line still inside the bottom region must match")
	}
}

// Faint text is an agent's dimmed ghost suggestion, not chrome it is drawing — the same
// rule IsStartupGate carries, on the same frame.
func TestIsStartupPicker_FaintTextIsNotChrome(t *testing.T) {
	esc := "\x1b"
	ghost := "❯ " + esc + "[2mResume from summary" + esc + "[0m\n"
	if IsStartupPicker(ghost, "") {
		t.Error("a faint suggestion is not a menu the agent is drawing")
	}
}

// WaitingOptions must keep excluding both kinds of startup chrome — this is the original
// "a reopened session reads waiting forever" false positive, and the refactor to a shared
// helper must not have loosened it.
func TestStartupChooserStillExcludedFromWaiting(t *testing.T) {
	for name, menu := range map[string]string{
		"resume picker": "  ❯ 1. Resume from summary (recommended)\n    2. Resume full session as-is\n",
		"trust gate":    "  Do you trust the files in this folder?\n\n  ❯ 1. Yes, proceed\n    2. No, exit\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := WaitingOptions(menu); got != nil {
				t.Errorf("WaitingOptions = %#v, want nil — session-startup chrome is not a task wait", got)
			}
		})
	}
}
