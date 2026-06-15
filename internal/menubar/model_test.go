package menubar

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	// Empty / no-server output → no agents, no error.
	for _, in := range []string{"", "  \n", "[]"} {
		got, err := Parse([]byte(in))
		if err != nil {
			t.Errorf("Parse(%q) error: %v", in, err)
		}
		if len(got) != 0 {
			t.Errorf("Parse(%q) = %v, want empty", in, got)
		}
	}

	// A real row decodes every contract field.
	js := `[{"pane_id":"%11","session":"Pica","window":"1","pane":"0","loc":"Pica:1.0",
	         "agent":"Claude Code","status":"waiting","task":"refactor auth","latest":true,"activity":false}]`
	got, err := Parse([]byte(js))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	want := Agent{PaneID: "%11", Session: "Pica", Window: "1", Pane: "0", Loc: "Pica:1.0",
		Agent: "Claude Code", Status: "waiting", Task: "refactor auth", Latest: true}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("Parse row =\n  %+v\nwant\n  %+v", got[0], want)
	}

	if _, err := Parse([]byte(`{not json`)); err == nil {
		t.Error("Parse(invalid) should error")
	}
}

func TestTitle(t *testing.T) {
	cases := []struct {
		name   string
		agents []Agent
		want   string
	}{
		{"empty is calm", nil, "✳"},
		{"all idle is calm", []Agent{{Status: "idle"}, {Status: "running"}}, "✳"},
		{"working shows count", []Agent{{Status: "working"}, {Status: "idle"}, {Status: "working"}}, "⠿2"},
		{"waiting wins over working", []Agent{{Status: "working"}, {Status: "waiting"}, {Status: "working"}}, "⏸1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Title(c.agents); got != c.want {
				t.Errorf("Title = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	if got := Summary(nil); got != "no agents" {
		t.Errorf("Summary(nil) = %q, want %q", got, "no agents")
	}
	agents := []Agent{{Status: "waiting"}, {Status: "working"}, {Status: "idle"}, {Status: "running"}}
	want := "4 agents · 1 waiting · 1 working · 2 idle" // running falls into the idle bucket
	if got := Summary(agents); got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
	if got := Summary([]Agent{{Status: "idle"}}); got != "1 agent · 0 working · 1 idle" {
		t.Errorf("Summary(1 idle) = %q", got)
	}
}

func TestRows(t *testing.T) {
	agents := []Agent{
		{PaneID: "%1", Session: "Pica", Loc: "Pica:1.0", Agent: "Claude Code", Status: "waiting", Task: "refactor auth"},
		{PaneID: "%2", Session: "work", Loc: "work:2.1", Agent: "Codex", Status: "working", Task: ""},
	}
	rows := Rows(agents)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].Label != "⏸ Pica · refactor auth" {
		t.Errorf("row0 label = %q", rows[0].Label)
	}
	if rows[0].PaneID != "%1" {
		t.Errorf("row0 paneID = %q, want %%1", rows[0].PaneID)
	}
	// Empty task falls back to the agent name.
	if rows[1].Label != "⠿ work · Codex" {
		t.Errorf("row1 label = %q, want fallback to agent name", rows[1].Label)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 48); got != "short" {
		t.Errorf("truncate(short) = %q", got)
	}
	long := "this is a very long task description that should get cut off somewhere"
	got := truncate(long, 20)
	if []rune(got)[len([]rune(got))-1] != '…' {
		t.Errorf("truncate should end with ellipsis, got %q", got)
	}
	if len([]rune(got)) != 20 {
		t.Errorf("truncate len = %d runes, want 20", len([]rune(got)))
	}
}

func TestResolveGtmux(t *testing.T) {
	// Explicit override wins.
	t.Setenv("GTMUX_BIN", "/custom/gtmux")
	if got := ResolveGtmux(); got != "/custom/gtmux" {
		t.Errorf("ResolveGtmux with override = %q, want /custom/gtmux", got)
	}

	// Sibling next to the executable is picked up.
	t.Setenv("GTMUX_BIN", "")
	dir := t.TempDir()
	sib := filepath.Join(dir, "gtmux")
	if err := os.WriteFile(sib, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// os.Executable() points at the test binary; emulate the sibling check directly.
	if !isExecutableFile(sib) {
		t.Error("expected the sibling to be detected as executable")
	}
	if isExecutableFile(filepath.Join(dir, "nope")) {
		t.Error("missing file should not be executable")
	}
}
