package tmux

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This uses a private socket and ignores user tmux configuration. No command
// can target the real server or an existing agent, even when the test fails.
func TestCaptureAlternateScreenBoundary(t *testing.T) {
	real, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux unavailable")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir, err := os.MkdirTemp("/tmp", "gtmux-screen-")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", dir)
	saved := Bin
	t.Cleanup(func() {
		_ = exec.Command(real, "-L", "boundary", "kill-server").Run()
		Bin = saved
		_ = os.RemoveAll(dir)
	})
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
	wrapper := filepath.Join(dir, "tmux")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec "+quote(real)+" -L boundary -f /dev/null \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	Bin = wrapper
	fixture := filepath.Join(dir, "screen.py")
	code := `import sys
sys.stdout.write(''.join('OLD-NORMAL-%d witness-before-alt\r\n' % n for n in range(40)))
sys.stdout.write('\x1b[?1049h\x1b[2J\x1b[H\x1b[32mCURRENT-当前\x1b[0m\r\n\r\n')
sys.stdout.flush()
for command in sys.stdin:
    if command.strip() == 'normal':
        sys.stdout.write('\x1b[?1049l\r\nNORMAL-RETURN\r\n')
    elif command.strip() == 'alternate':
        sys.stdout.write('\x1b[?1049h\x1b[2J\x1b[H\x1b[32mCURRENT-当前\x1b[0m\r\n\r\n')
    sys.stdout.flush()
`
	if err := os.WriteFile(fixture, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	id, err := Run("new-session", "-d", "-P", "-F", "#{pane_id}", "-x", "80", "-y", "12", "-s", "capture", quote(python)+" -u "+quote(fixture))
	if err != nil {
		t.Fatalf("private tmux server: %v", err)
	}
	wait := func(mode, marker string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if Display(id, "#{alternate_on}") == mode && strings.Contains(CapturePane(id), marker) {
				return
			}
			time.Sleep(15 * time.Millisecond)
		}
		t.Fatalf("private fixture did not reach mode %s, marker %s", mode, marker)
	}
	assertAlternate := func() {
		t.Helper()
		wait("1", "CURRENT-当前")
		before := Display(id, "#{history_size}")
		old, _ := runRaw("capture-pane", "-p", "-e", "-S", "-2000", "-t", id)
		if !strings.Contains(old, "OLD-NORMAL") || !strings.Contains(old, "CURRENT-当前") {
			t.Fatal("fixture did not reproduce the stale normal-history/current-frame join")
		}
		for _, text := range []string{CapturePaneColor(id), CaptureFull(id), CaptureFullColor(id)} {
			if strings.Contains(text, "witness-before-alt") || !strings.Contains(text, "CURRENT-当前") {
				t.Fatalf("capture crossed a screen boundary: %q", text)
			}
		}
		text := CapturePaneColor(id)
		height, _ := strconv.Atoi(Display(id, "#{pane_height}"))
		if !strings.Contains(text, "\x1b[32m") || strings.Count(text, "\n") != height {
			t.Fatalf("ANSI or trailing grid rows changed: %q (height %d)", text, height)
		}
		if Display(id, "#{history_size}") != before {
			t.Fatal("read-only capture changed terminal history")
		}
	}
	assertAlternate()
	if err := SendText(id, "normal", true); err != nil {
		t.Fatal(err)
	}
	wait("0", "NORMAL-RETURN")
	if text := CapturePaneColor(id); !strings.Contains(text, "OLD-NORMAL") || !strings.Contains(text, "NORMAL-RETURN") {
		t.Fatalf("normal-screen scrollback disappeared: %q", text)
	}
	// A target name is data, even when it contains tmux command delimiters.
	name := "quoted'; display-message -p INJECTED"
	if _, err := Run("rename-session", "-t", "capture", name); err != nil {
		t.Fatal(err)
	}
	if text := CapturePaneColor(name + ":0.0"); !strings.Contains(text, "NORMAL-RETURN") || strings.Contains(text, "INJECTED") {
		t.Fatalf("target quoting lost the pane or executed its name: %q", text)
	}
	if err := SendText(id, "alternate", true); err != nil {
		t.Fatal(err)
	}
	assertAlternate()
	if got := CapturePaneColor("%99999999"); got != "" {
		t.Fatalf("missing target fell back to another pane: %q", got)
	}
}
