package ghostty

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// TestQuote: escape backslashes then double-quotes for an AppleScript literal.
func TestQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"plain", "plain"},
		{`a"b`, `a\"b`},
		{`a\b`, `a\\b`},
		{`"`, `\"`},
		{`\`, `\\`},
		{`\"`, `\\\"`}, // backslash escaped first, then the quote
		{`he said "hi"`, `he said \"hi\"`},
		{`C:\path\"x"`, `C:\\path\\\"x\"`},
		{"日常更新", "日常更新"}, // CJK untouched
	}
	for _, c := range cases {
		if got := Quote(c.in); got != c.want {
			t.Errorf("Quote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestShellQuote: single-quote wrapping with the '\” escape for embedded quotes.
func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"plain", "'plain'"},
		{"a b", "'a b'"},
		{"it's", `'it'\''s'`},
		{"'", `''\'''`},
		{"a'b'c", `'a'\''b'\''c'`},
	}
	for _, c := range cases {
		if got := ShellQuote(c.in); got != c.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSessionsFromTitlesEdges covers ordering/dedup/whitespace/edge inputs not in
// the existing happy-path test.
func TestSessionsFromTitlesEdges(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string yields nothing", "", nil},
		{"only blank lines", "\n  \n\t\n", nil},
		{"no separator anywhere", "plain\nbash prompt\n", nil},
		{"trims surrounding whitespace", "  Foo — Bar  \n", []string{"Foo"}},
		{"dedup preserves first-seen order", "B — x\nA — y\nB — z\nA — w\n", []string{"B", "A"}},
		{"blank session name before sep is skipped", " — only window\n", nil},
		{"separator must be space-emdash-space, not hyphen", "Foo - Bar\n", nil},
		{"trailing/leading separators within session trimmed", "  Sess  — W\n", []string{"Sess"}},
		{"no trailing newline still parses last line", "Solo — W", []string{"Solo"}},
		{"CRLF: carriage return is trimmed off the window, session intact", "Foo — Bar\r\n", []string{"Foo"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SessionsFromTitles(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("SessionsFromTitles(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestGhosttyTabScript: the native-jump script targets Ghostty, matches the tab
// by exact title, and quotes the title against injection.
func TestGhosttyTabScript(t *testing.T) {
	s := ghosttyTabScript("My Tab")
	for _, want := range []string{
		`tell application "Ghostty"`,
		`if name of t is "My Tab" then`,
		`select tab t`,
		`activate window w`,
		`return "ok"`,
		`return "notfound"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("ghosttyTabScript missing %q in:\n%s", want, s)
		}
	}
	// Quoting: a title with a double-quote must be escaped so it can't break out.
	q := ghosttyTabScript(`evil" then activate (`)
	if !strings.Contains(q, `name of t is "evil\" then activate (" then`) {
		t.Errorf("ghosttyTabScript did not escape the title:\n%s", q)
	}
}

// TestWindowScript: the new-window script activates Ghostty, builds a surface
// configuration, sets the command, and quotes it.
func TestWindowScript(t *testing.T) {
	s := windowScript("echo hi")
	for _, want := range []string{
		`tell application "Ghostty"`,
		`activate`,
		`set cfg to new surface configuration`,
		`set command of cfg to "echo hi"`,
		`new window with configuration cfg`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("windowScript missing %q in:\n%s", want, s)
		}
	}
	// The command is Quote()'d so embedded quotes can't break the literal.
	q := windowScript(`gtmux overview "x"`)
	if !strings.Contains(q, `set command of cfg to "gtmux overview \"x\""`) {
		t.Errorf("windowScript did not escape the command:\n%s", q)
	}
}

// TestSpawnTabsDryRun asserts the generated AppleScript for spawning one tab per
// session, without executing osascript (dryRun=true).
func TestSpawnTabsDryRun(t *testing.T) {
	script, err := SpawnTabs([]string{"alpha", "two words"}, true)
	if err != nil {
		t.Fatalf("dry-run SpawnTabs returned error: %v", err)
	}
	for _, want := range []string{
		"tell application \"Ghostty\"",
		"activate",
		"if (count of windows) is 0 then",
		"new window with configuration cfg",
		"new tab in front window with configuration cfg",
		"end tell",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("SpawnTabs script missing %q in:\n%s", want, script)
		}
	}
	// Each session becomes a `tmux attach -t '<shell-quoted name>'` command,
	// AppleScript-quoted inside the surface config.
	wantAlpha := Quote(tmux.Bin + " attach -t " + ShellQuote("alpha"))
	if !strings.Contains(script, `set command of cfg to "`+wantAlpha+`"`) {
		t.Errorf("SpawnTabs missing attach command for alpha (%q) in:\n%s", wantAlpha, script)
	}
	// Session names with spaces are ShellQuote'd so attach targets the right name.
	wantTwo := Quote(tmux.Bin + " attach -t " + ShellQuote("two words"))
	if !strings.Contains(script, `set command of cfg to "`+wantTwo+`"`) {
		t.Errorf("SpawnTabs missing attach command for 'two words' (%q) in:\n%s", wantTwo, script)
	}
	// One surface-config block per session.
	if n := strings.Count(script, "set cfg to new surface configuration"); n != 2 {
		t.Errorf("expected 2 surface-config blocks, got %d", n)
	}
}

// TestSpawnTabsEmpty: with no sessions, the script is just the tell/activate
// wrapper (no per-session blocks).
func TestSpawnTabsEmpty(t *testing.T) {
	script, err := SpawnTabs(nil, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.Contains(script, "surface configuration") {
		t.Errorf("empty SpawnTabs should emit no surface blocks:\n%s", script)
	}
	hasPrefix := strings.HasPrefix(script, "tell application \"Ghostty\"")
	hasSuffix := strings.HasSuffix(script, "end tell")
	if !hasPrefix || !hasSuffix {
		t.Errorf("empty SpawnTabs should still be a well-formed tell block:\n%s", script)
	}
}

// TestDriverName pins the public driver identity (used by terminal selection).
func TestDriverName(t *testing.T) {
	if got := (Driver{}).Name(); got != "Ghostty" {
		t.Errorf("Driver.Name() = %q, want Ghostty", got)
	}
}

// TestSessionsFromTitlesStripsBellGlyph: Ghostty prepends a bell/activity glyph to
// a background tab's title; the session-name extraction must strip it so tab-order
// can still match the session.
func TestSessionsFromTitlesStripsBellGlyph(t *testing.T) {
	got := strings.Join(SessionsFromTitles("Diting — Diting Dev\n🔔 ccy-workspace — ccy\n日常更新 — kb\n"), ",")
	if got != "Diting,ccy-workspace,日常更新" {
		t.Fatalf("bell glyph not stripped: %q", got)
	}
}
