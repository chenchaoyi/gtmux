package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The warp driver must be registered under the name detect.go resolves for a
// Warp-hosted tmux client — otherwise focus silently falls back to the Ghostty
// driver and targets the wrong app (the pre-driver bug, verified live).
func TestWarpRegistered(t *testing.T) {
	if !HasDriver("warp") {
		t.Fatal("no driver registered for \"warp\"")
	}
	if got := registry["warp"].Name(); got != "Warp" {
		t.Fatalf("warp driver Name() = %q, want %q", got, "Warp")
	}
}

// detect.go must map both the in-terminal env and a Warp process ancestry to
// the registered driver name (real values from a live Warp v0.2026.06).
func TestWarpDetection(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "WarpTerminal")
	if got := fromTermEnv(); got != "warp" {
		t.Fatalf("fromTermEnv() = %q, want warp", got)
	}
	// The tmux client's ancestor on a real Mac: Warp's terminal-server process.
	if got := terminalFromCommand("/Applications/Warp.app/Contents/MacOS/stable terminal-server --parent-pid=5418"); got != "warp" {
		t.Fatalf("terminalFromCommand(warp terminal-server) = %q, want warp", got)
	}
}

func TestHostAppNameDisplay(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "WarpTerminal")
	if got := HostAppName(); got != "Warp" {
		t.Fatalf("HostAppName() under TERM_PROGRAM=WarpTerminal = %q, want Warp", got)
	}
	t.Setenv("TERM_PROGRAM", "ghostty")
	if got := HostAppName(); got != "Ghostty" {
		t.Fatalf("HostAppName() under TERM_PROGRAM=ghostty = %q, want Ghostty", got)
	}
}

func TestWarpViewingMatch(t *testing.T) {
	cases := []struct {
		name, out, session string
		want               bool
	}{
		{"warp frontmost, exact title", "stable\nmysess", "mysess", true},
		{"warp frontmost, titled tab", "stable\nmysess — vim", "mysess", true},
		{"future process name", "Warp\nmysess — 0", "mysess", true},
		{"other app frontmost", "ghostty\nmysess — vim", "mysess", false},
		{"warp frontmost, other session", "stable\nother — vim", "mysess", false},
		{"warp frontmost, no osc title", "stable\n", "mysess", false},
		{"prefix is not a word match", "stable\nmysess2 — vim", "mysess", false},
		{"malformed output", "stable", "mysess", false},
	}
	for _, c := range cases {
		if got := warpViewingMatch(c.out, c.session); got != c.want {
			t.Errorf("%s: warpViewingMatch(%q, %q) = %v, want %v", c.name, c.out, c.session, got, c.want)
		}
	}
}

func TestWarpLaunchYAML(t *testing.T) {
	oldCwd := warpLaunchCwd
	warpLaunchCwd = func() string { return "/Users/x" }
	defer func() { warpLaunchCwd = oldCwd }()
	y := warpLaunchYAML("gtmux-restore", [][2]string{
		{"dev", "bash /x/gtmux-attach.sh 'dev'"},
		{`we"ird`, `bash /x/gtmux-attach.sh 'we"ird'`},
	})
	for _, want := range []string{
		"name: gtmux-restore",
		"- title: \"dev\"",
		// Warp v0.2026.08+ REJECTS a pane template without cwd (must be an
		// absolute path) — omitting it makes the whole config invisible to
		// warp://launch, so restore/new silently open nothing (verified live).
		"cwd: \"/Users/x\"",
		"- exec: \"bash /x/gtmux-attach.sh 'dev'\"",
		"- title: \"we\\\"ird\"", // quotes escaped for the double-quoted YAML scalar
	} {
		if !strings.Contains(y, want) {
			t.Errorf("launch YAML missing %q:\n%s", want, y)
		}
	}
	// cwd must appear once per tab, before its commands.
	if got := strings.Count(y, "cwd: \"/Users/x\""); got != 2 {
		t.Errorf("want cwd in each of 2 tab layouts, got %d:\n%s", got, y)
	}
}

// SpawnTabs (dry-run exercises the YAML; a real run also writes the attach
// script that records each tab's Warp session uuid into the tmux session env —
// the only per-tab focus handle Warp offers, since its tabs aren't scriptable
// and macOS no longer exposes other processes' env).
func TestWarpSpawnTabs(t *testing.T) {
	dir := t.TempDir()
	old := launchConfigDir
	launchConfigDir = func() string { return dir }
	defer func() { launchConfigDir = old }()

	opened := ""
	oldOpen := openURL
	openURL = func(url string) error { opened = url; return nil }
	defer func() { openURL = oldOpen }()

	y, err := warp{}.SpawnTabs([]string{"alpha", "beta"}, false)
	if err != nil {
		t.Fatalf("SpawnTabs: %v", err)
	}
	if opened != "warp://launch/gtmux-restore" {
		t.Fatalf("opened %q, want warp://launch/gtmux-restore", opened)
	}
	for _, want := range []string{"- title: \"alpha\"", "- title: \"beta\"", "gtmux-attach.sh"} {
		if !strings.Contains(y, want) {
			t.Errorf("SpawnTabs YAML missing %q:\n%s", want, y)
		}
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "gtmux-restore.yaml"))
	if err != nil {
		t.Fatalf("launch config not written: %v", err)
	}
	if string(cfg) != y {
		t.Error("written launch config differs from returned YAML")
	}
	script, err := os.ReadFile(filepath.Join(dir, "gtmux-attach.sh"))
	if err != nil {
		t.Fatalf("attach script not written: %v", err)
	}
	for _, want := range []string{"attach -t \"$s\"", "set-environment -t \"$s\" WARP_TERMINAL_SESSION_UUID"} {
		if !strings.Contains(string(script), want) {
			t.Errorf("attach script missing %q:\n%s", want, script)
		}
	}
}

func TestWarpOpenWindow(t *testing.T) {
	dir := t.TempDir()
	old := launchConfigDir
	launchConfigDir = func() string { return dir }
	defer func() { launchConfigDir = old }()

	opened := ""
	oldOpen := openURL
	openURL = func(url string) error { opened = url; return nil }
	defer func() { openURL = oldOpen }()

	if _, err := (warp{}).OpenWindow("gtmux hq"); err != nil {
		t.Fatalf("OpenWindow: %v", err)
	}
	if opened != "warp://launch/gtmux-window" {
		t.Fatalf("opened %q, want warp://launch/gtmux-window", opened)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "gtmux-window.yaml"))
	if err != nil {
		t.Fatalf("launch config not written: %v", err)
	}
	if !strings.Contains(string(cfg), "- exec: \"gtmux hq\"") {
		t.Errorf("launch config missing the command:\n%s", cfg)
	}
}

func TestWarpTabOrderNil(t *testing.T) {
	if got := (warp{}).TabOrder(); got != nil {
		t.Fatalf("TabOrder() = %v, want nil (Warp tabs aren't scriptable)", got)
	}
}
