package hook

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestCodexPaneForCwd(t *testing.T) {
	panes := []codexPane{
		{"%16", "codex", "/work/site", "site:0.0"},
		{"%21", "codex", "/work/hq", "hq:0.0"},
		{"%22", "bash", "/work/hq", "hq:0.1"},
	}
	for _, tc := range []struct {
		name, inherited, cwd, want string
	}{
		{"stale inherited pane", "%16", "/work/hq", "%21"},
		{"matching pane", "%16", "/work/site", "%16"},
		{"missing pane", "%16", "/work/other", ""},
		{"no payload cwd", "%16", "", "%16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := codexPaneForCwd(tc.inherited, tc.cwd, "", panes, nil); got != tc.want {
				t.Fatalf("pane = %q, want %q", got, tc.want)
			}
		})
	}
	panes = append(panes, codexPane{"%23", "codex", "/work/hq", "hq:0.2"})
	if got := codexPaneForCwd("%21", "/work/hq", "", panes, nil); got != "" {
		t.Fatalf("ambiguous cwd chose %q", got)
	}
}

func TestCodexPaneForCwdUsesUniqueSessionBinding(t *testing.T) {
	panes := []codexPane{
		{"%16", "codex", "/work/hq", "hq:0.0"},
		{"%21", "codex", "/work/hq", "hq:0.1"},
	}
	if got := codexPaneForCwd("%16", "/work/hq", "session-b", panes, map[string]string{"%21": "session-b"}); got != "%21" {
		t.Fatalf("session binding chose %q, want %%21", got)
	}
	if got := codexPaneForCwd("%16", "/work/hq", "session-x", panes, nil); got != "" {
		t.Fatalf("unbound ambiguous session chose %q", got)
	}
}

func TestCodexBoundSessionsRequireCodexAndMatchingCwd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := resume.Save("hq:0.0", resume.Record{Agent: "codex", SessionID: "session-a", Cwd: "/work/a"}); err != nil {
		t.Fatal(err)
	}
	if err := resume.Save("hq:0.1", resume.Record{Agent: "claude", SessionID: "session-b", Cwd: "/work/a"}); err != nil {
		t.Fatal(err)
	}
	if err := resume.Save("hq:0.2", resume.Record{Agent: "codex", SessionID: "session-c", Cwd: "/work/old"}); err != nil {
		t.Fatal(err)
	}
	got := codexBoundSessions([]codexPane{
		{"%1", "codex", "/work/a", "hq:0.0"},
		{"%2", "codex", "/work/a", "hq:0.1"},
		{"%3", "codex", "/work/a", "hq:0.2"},
	})
	if len(got) != 1 || got["%1"] != "session-a" {
		t.Fatalf("bindings = %#v, want only %%1 → session-a", got)
	}
}
