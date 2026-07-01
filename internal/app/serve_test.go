package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/server"
)

func TestResolveServeTokenFlagWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := resolveServeToken("explicit"); got != "explicit" {
		t.Fatalf("flag token = %q, want explicit", got)
	}
	// An explicit token must not write the persistent file.
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "serve-token")); !os.IsNotExist(err) {
		t.Fatalf("explicit token should not persist a file")
	}
}

func TestResolveServeTokenGeneratesAndPersists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	first := resolveServeToken("")
	if len(first) < 16 {
		t.Fatalf("generated token too short: %q", first)
	}
	path := filepath.Join(home, ".config", "gtmux", "serve-token")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("token file not written: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("token file perm = %o, want 600", perm)
	}

	// A second call reuses the persisted token (stable across restarts).
	if second := resolveServeToken(""); second != first {
		t.Fatalf("token not stable: %q != %q", second, first)
	}
}

func TestReachableHostsSpecificBind(t *testing.T) {
	hosts := reachableHosts("10.0.0.5")
	if len(hosts) != 1 || hosts[0] != "10.0.0.5" {
		t.Fatalf("specific bind hosts = %v, want [10.0.0.5]", hosts)
	}
}

func TestReachableHostsWildcardNonEmpty(t *testing.T) {
	// Wildcard expands to interface IPs; on any host there is at least the
	// fallback, so the slice is never empty (avoids an empty banner).
	if hosts := reachableHosts("0.0.0.0"); len(hosts) == 0 {
		t.Fatalf("wildcard hosts must not be empty")
	}
}

// TestCursorFromFields covers the bottom-anchored cursor math (Up =
// pane_height-1-cursor_y, clamped) and the reject paths, without a running tmux.
func TestCursorFromFields(t *testing.T) {
	cases := []struct {
		name            string
		fields          []string
		wantX, wantUp   int
		wantVis, wantOK bool
	}{
		{"bottom row", []string{"4", "23", "24", "1"}, 4, 0, true, true}, // cy at last row → up 0
		{"three up", []string{"0", "20", "24", "1"}, 0, 3, true, true},   // 24-1-20 = 3
		{"hidden cursor", []string{"7", "23", "24", "0"}, 7, 0, false, true},
		{"clamp negative", []string{"2", "30", "24", "1"}, 2, 0, true, true}, // cy past bottom → clamp 0
		{"wrong arity", []string{"4", "23", "24"}, 0, 0, false, false},
		{"non-numeric", []string{"x", "23", "24", "1"}, 0, 0, false, false},
		{"zero height", []string{"4", "0", "0", "1"}, 0, 0, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			x, up, vis, ok := cursorFromFields(c.fields)
			if x != c.wantX || up != c.wantUp || vis != c.wantVis || ok != c.wantOK {
				t.Fatalf("cursorFromFields(%v) = (%d,%d,%v,%v), want (%d,%d,%v,%v)",
					c.fields, x, up, vis, ok, c.wantX, c.wantUp, c.wantVis, c.wantOK)
			}
		})
	}
}

// pushCopy: the TITLE is the agent's session name (its task), NOT "<Agent> needs
// you"; the state (needs you / still / finished) is the body. Falls back to the
// locator when the task is empty.
func TestPushCopyTitleIsSessionName(t *testing.T) {
	noPane := func(string) (string, bool) { return "", false } // no parseable menu
	waiting := server.Alert{Kind: "waiting", Agent: "Claude Code", Task: "gtmux.app dev", Loc: "ws:1.0", Pane: "%1"}
	if title, body := pushCopy(waiting, noPane); title != "gtmux.app dev" || body == "" || title == body {
		t.Errorf("waiting → title=%q body=%q; want title=session name, body=state", title, body)
	}
	done := server.Alert{Kind: "done", Agent: "Codex", Task: "fix build", Loc: "ws:2.0"}
	if title, _ := pushCopy(done, noPane); title != "fix build" {
		t.Errorf("done title = %q, want the task name", title)
	}
	// empty task → fall back to the locator, never "<Agent> needs you"
	if title, _ := pushCopy(server.Alert{Kind: "waiting", Agent: "Claude Code", Loc: "ws:3.0"}, noPane); title != "ws:3.0" {
		t.Errorf("empty-task title = %q, want the loc fallback", title)
	}
	// #3: a waiting pane's real 1/2/3 choices become the body.
	withMenu := func(string) (string, bool) { return "❯ 1. Yes\n  2. Always, don't ask\n  3. No", true }
	if _, body := pushCopy(waiting, withMenu); body != "1. Yes   2. Always, don't ask   3. No" {
		t.Errorf("waiting body = %q, want the real options list", body)
	}
}
