package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestSnip(t *testing.T) {
	if got := snip("", 10); got != "" {
		t.Errorf("snip empty = %q", got)
	}
	if got := snip("a  b\n\tc", 10); got != "a b c" {
		t.Errorf("snip whitespace = %q, want %q", got, "a b c")
	}
	long := strings.Repeat("字", 30)
	got := snip(long, 10)
	if !strings.HasSuffix(got, "…") || len([]rune(got)) != 11 {
		t.Errorf("snip rune truncation = %q (runes %d)", got, len([]rune(got)))
	}
}

func TestJoinAsk(t *testing.T) {
	if got := joinAsk(nil); got != "" {
		t.Errorf("joinAsk(nil) = %q", got)
	}
	got := joinAsk([]prompt.Option{{N: 1, Label: "Yes"}, {N: 2, Label: "No, tell Claude what to do"}})
	want := "1.Yes · 2.No, tell Claude what to do"
	if got != want {
		t.Errorf("joinAsk = %q, want %q", got, want)
	}
}

func TestTurnDigest(t *testing.T) {
	if g, l := turnDigest(nil); g != "" || l != "" {
		t.Errorf("empty turns = (%q,%q)", g, l)
	}
	turns := []transcript.Turn{
		{Prompt: "old", Response: "old reply"},
		{Prompt: "fix the login bug", Response: "Found it.\nThe token was expired — patched and tests pass."},
	}
	g, l := turnDigest(turns)
	if g != "fix the login bug" {
		t.Errorf("goal = %q", g)
	}
	if !strings.Contains(l, "tests pass") {
		t.Errorf("last should carry the reply tail, got %q", l)
	}
	// A very long reply keeps its TAIL (the current end), marked with a leading …
	long := transcript.Turn{Prompt: "p", Response: strings.Repeat("x ", 500) + "THE END"}
	_, l = turnDigest([]transcript.Turn{long})
	if !strings.HasPrefix(l, "…") || !strings.HasSuffix(l, "THE END") {
		t.Errorf("long reply should tail-truncate, got %q…%q", l[:10], l[len(l)-10:])
	}
}

// roleForCwd marks ONLY the hq home; "" and other dirs stay unmarked.
func TestRoleForCwd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := roleForCwd(state.HQHome()); got != "supervisor" {
		t.Errorf("hq home role = %q, want supervisor", got)
	}
	if got := roleForCwd(""); got != "" {
		t.Errorf("empty cwd role = %q", got)
	}
	if got := roleForCwd(filepath.Join(os.Getenv("HOME"), "code")); got != "" {
		t.Errorf("other cwd role = %q", got)
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
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
	rows := []digestRow{
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
		row  digestRow
		want string
	}{
		{"waiting", digestRow{Status: "waiting"}, "needs_input"},
		{"idle", digestRow{Status: "idle"}, "completed"},
		{"working", digestRow{Status: "working"}, "working"},
		{"running folds into working", digestRow{Status: "running"}, "working"},
		{"error wins over idle", digestRow{Status: "idle", Error: "boom"}, "errored"},
		{"error wins over waiting", digestRow{Status: "waiting", Error: "boom"}, "errored"},
	}
	for _, c := range cases {
		if got := digestBucket(c.row); got != c.want {
			t.Errorf("%s: digestBucket = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDigestBadge(t *testing.T) {
	if got := digestBadge(digestRow{Task: "t", TaskStatus: "done"}); got != "done" {
		t.Errorf("task badge = %q, want %q", got, "done")
	}
	if got := digestBadge(digestRow{Ask: "1.Yes · 2.No · 3.Maybe"}); got != "3 opts" {
		t.Errorf("ask badge = %q, want %q", got, "3 opts")
	}
	if got := digestBadge(digestRow{UsageWarn: "ctx 92%"}); got != "⚠" {
		t.Errorf("usage-warn badge = %q, want %q", got, "⚠")
	}
	if got := digestBadge(digestRow{}); got != "" {
		t.Errorf("empty row badge = %q, want empty", got)
	}
}
