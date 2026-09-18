package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

func TestAgentsSummary(t *testing.T) {
	defer i18n.SetLang("en")
	i18n.SetLang("en")

	if got := agentsSummary(nil); got != "0 agents" {
		t.Errorf("empty summary = %q, want %q", got, "0 agents")
	}
	panes := []radar.Pane{
		{Status: "waiting"}, {Status: "working"}, {Status: "idle"}, {Status: "running"},
	}
	want := "4 agents · 1 waiting · 1 working · 2 idle" // running counts toward idle bucket
	if got := agentsSummary(panes); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

// readmeFleet is the sample fleet both READMEs show, and the same one the phone, iPad,
// browser and menu-bar screenshots are taken from: one agent waiting, the supervisor and
// one worker moving, three finished, one bare shell. No row carries a background job or
// an error — those modifiers rewrite the task column, and the opening picture is there to
// show the ordinary day.
func readmeFleet() []radar.Pane {
	return []radar.Pane{
		{PaneID: "%7", Loc: "api:0.0", Agent: "Claude Code", Status: "waiting", Task: "permission to run tests"},
		{PaneID: "%1", Loc: "hq:0.0", Agent: "Claude Code", Status: "working", Task: "api is waiting on you · rest normal"},
		{PaneID: "%11", Loc: "web:0.0", Agent: "Claude Code", Status: "working", Task: "refactor auth middleware"},
		{PaneID: "%9", Loc: "app:0.0", Agent: "Claude Code", Status: "idle", Task: "wire up the dashboard"},
		{PaneID: "%8", Loc: "worker:0.0", Agent: "Codex", Status: "idle", Task: "add retry backoff", Latest: true},
		{PaneID: "%3", Loc: "docs:0.0", Agent: "Gemini", Status: "idle", Task: "draft the API reference"},
		{PaneID: "%5", Loc: "infra:0.0", Agent: "Claude Code", Status: "running"},
	}
}

// The README opens on a picture of `gtmux agents`, and for months that picture was not
// what the command prints: an em dash where the code writes "·", and a closing
// "jump: gtmux focus %7" in place of the real hint. A reader's first impression of the
// product was a line it cannot produce. Nothing compared the two, so nothing complained.
//
// Both halves are checked, each in its own language, because the CLI answers in the
// reader's: a Chinese README showing English labels would be just as wrong.
func TestREADMEAgentsSampleIsReal(t *testing.T) {
	defer i18n.SetLang("en")
	for _, tc := range []struct{ lang, file string }{{"en", "README.md"}, {"zh", "README.zh.md"}} {
		i18n.SetLang(tc.lang)
		want := stripANSI(agentsTable(readmeFleet()))
		body, err := os.ReadFile(filepath.Join("..", "..", tc.file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), want) {
			t.Errorf("%s does not show what `gtmux agents` prints. Paste this block in:\n%s", tc.file, want)
		}
	}
}

// stripANSI drops the styling so the doc block can be compared as the text it is.
func stripANSI(s string) string { return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(s, "") }
