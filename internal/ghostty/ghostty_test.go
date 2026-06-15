package ghostty

import (
	"strings"
	"testing"
)

func TestQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{`say "hi"`, `say \"hi\"`},
		{`a\b`, `a\\b`},
		{`a"b\c`, `a\"b\\c`}, // backslash escaped first, then quote
	}
	for _, c := range cases {
		if got := Quote(c.in); got != c.want {
			t.Errorf("Quote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", "'plain'"},
		{"my session", "'my session'"},
		{"it's", `'it'\''s'`},
	}
	for _, c := range cases {
		if got := ShellQuote(c.in); got != c.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWindowScript(t *testing.T) {
	s := windowScript(`/Apps/Gtmux.app/Contents/MacOS/gtmux overview --hold`)
	for _, w := range []string{
		`tell application "Ghostty"`,
		"new window with configuration cfg",
		"gtmux overview --hold",
		"end tell",
	} {
		if !strings.Contains(s, w) {
			t.Errorf("windowScript missing %q\n--- script ---\n%s", w, s)
		}
	}
	// A command with a quote must be escaped into the AppleScript literal.
	if got := windowScript(`say "hi"`); !strings.Contains(got, `say \"hi\"`) {
		t.Errorf("windowScript did not escape quotes: %s", got)
	}
}

// SpawnTabs in dryRun must build a valid-looking AppleScript without executing
// osascript, and must shell-quote each session name in its tmux attach command.
func TestSpawnTabsDryRun(t *testing.T) {
	script, err := SpawnTabs([]string{"alpha", "my sess"}, true)
	if err != nil {
		t.Fatalf("SpawnTabs dryRun error: %v", err)
	}
	wants := []string{
		`tell application "Ghostty"`,
		"attach -t 'alpha'",
		"attach -t 'my sess'",
		"new tab in front window",
		"end tell",
	}
	for _, w := range wants {
		if !strings.Contains(script, w) {
			t.Errorf("SpawnTabs script missing %q\n--- script ---\n%s", w, script)
		}
	}
	// One surface configuration per session.
	if n := strings.Count(script, "new surface configuration"); n != 2 {
		t.Errorf("got %d surface configurations, want 2", n)
	}
}
