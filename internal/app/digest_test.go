package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/radar"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// The formatted digest report (RULE from CLAUDE.md: HQ's status report must be a
// column-aligned scannable table, not prose) — a top summary-by-state line, one
// section per state (needs-you first), and one aligned row per agent with a
// status glyph, a name, a truncated goal/last/ask, a right badge, and a
// right-aligned relative time. Pins the shape so a future edit can't silently
// regress it back to a paragraph dump.
func TestRenderDigestTable(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	now := time.Now()
	rows := []radar.DigestRow{
		{Loc: "proj:1.0", Status: "waiting", Ask: "1.Yes · 2.No", Since: now.Add(-30 * time.Second).Unix()},
		{Loc: "proj:2.0", Status: "working", Goal: "fix the login bug", Task: "fix the login bug", TaskStatus: "working", Since: now.Add(-2 * time.Minute).Unix()},
		{Loc: "proj:3.0", Status: "idle", Last: "done, tests pass", Since: now.Add(-2 * 24 * time.Hour).Unix()},
		{Loc: "proj:4.0", Status: "idle", Error: "panic: nil pointer", Since: now.Add(-10 * time.Minute).Unix()},
		{Loc: "proj:5.0", Status: "running", UsageWarn: "ctx 92%", Since: now.Add(-3 * time.Hour).Unix()},
	}
	out := captureStdout(t, func() { renderDigestTable(rows) })

	if !strings.Contains(out, "1 needs input") {
		t.Errorf("summary line missing needs-input count:\n%s", out)
	}
	if !strings.Contains(out, "2 working") {
		t.Errorf("summary line missing working count (working+running fold together):\n%s", out)
	}
	if !strings.Contains(out, "1 completed") {
		t.Errorf("summary line missing completed count:\n%s", out)
	}
	if !strings.Contains(out, "1 errored") {
		t.Errorf("summary line missing errored count:\n%s", out)
	}
	for _, section := range []string{"needs input (1)", "working (2)", "completed (1)", "errored (1)"} {
		if !strings.Contains(out, section) {
			t.Errorf("missing section header %q:\n%s", section, out)
		}
	}
	for _, want := range []string{"proj:1.0", "proj:2.0", "proj:3.0", "proj:4.0", "proj:5.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing row for %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "panic: nil pointer") {
		t.Errorf("errored row should surface its error text in the middle column:\n%s", out)
	}
	// Right-side badges: dispatch status, ask-option count, usage warning.
	for _, want := range []string{"2 opts", "working", "⚠"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing badge %q:\n%s", want, out)
		}
	}
	// Right-aligned compact relative time.
	for _, want := range []string{"30s", "2m", "10m", "2d", "3h"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing relative time %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "goal:") || strings.Contains(out, "目标:") {
		t.Errorf("table format should not carry the old label:value prose lines:\n%s", out)
	}
}

func TestDigestBucket(t *testing.T) {
	cases := []struct {
		name string
		row  radar.DigestRow
		want string
	}{
		{"waiting", radar.DigestRow{Status: "waiting"}, "needs_input"},
		{"idle", radar.DigestRow{Status: "idle"}, "completed"},
		{"working", radar.DigestRow{Status: "working"}, "working"},
		{"running folds into working", radar.DigestRow{Status: "running"}, "working"},
		{"error wins over idle", radar.DigestRow{Status: "idle", Error: "boom"}, "errored"},
		{"error wins over waiting", radar.DigestRow{Status: "waiting", Error: "boom"}, "errored"},
	}
	for _, c := range cases {
		if got := digestBucket(c.row); got != c.want {
			t.Errorf("%s: digestBucket = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDigestBadge(t *testing.T) {
	if got := digestBadge(radar.DigestRow{Task: "t", TaskStatus: "done"}); got != "done" {
		t.Errorf("task badge = %q, want %q", got, "done")
	}
	if got := digestBadge(radar.DigestRow{Ask: "1.Yes · 2.No · 3.Maybe"}); got != "3 opts" {
		t.Errorf("ask badge = %q, want %q", got, "3 opts")
	}
	if got := digestBadge(radar.DigestRow{UsageWarn: "ctx 92%"}); got != "⚠" {
		t.Errorf("usage-warn badge = %q, want %q", got, "⚠")
	}
	if got := digestBadge(radar.DigestRow{}); got != "" {
		t.Errorf("empty row badge = %q, want empty", got)
	}
}

// The digest --json / GET /api/digest contract carries in_mode so the supervisor
// sees which pane is input-locked (copy/view-mode); a normal pane omits it.
func TestDigestInModeContract(t *testing.T) {
	mb, _ := json.Marshal(radar.DigestRow{PaneID: "%3", Status: "waiting", Source: "tmux", InMode: true})
	if !strings.Contains(string(mb), `"in_mode":true`) {
		t.Errorf("input-locked digest row missing in_mode: %s", mb)
	}
	nb, _ := json.Marshal(radar.DigestRow{PaneID: "%4", Status: "idle", Source: "tmux"})
	if strings.Contains(string(nb), `"in_mode"`) {
		t.Errorf("non-locked digest row should omit in_mode: %s", nb)
	}
}
