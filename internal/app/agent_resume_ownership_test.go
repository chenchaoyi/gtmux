package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// The live-owner regression must assert what restore actually types, on a private server.
func TestRestoreConversationOwnership(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	for _, tc := range []struct {
		name                   string
		live, exact, ambiguous bool
		want                   string
	}{
		{name: "live owner and stale alias", live: true},
		{name: "live owner and duplicate exact record", live: true, exact: true},
		{name: "ambiguous historical candidates", ambiguous: true},
		{name: "unique renamed locator", want: "thread-a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sock, err := os.MkdirTemp("/tmp", "gtown")
			if err != nil {
				t.Fatal(err)
			}
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
			t.Setenv("TMUX_TMPDIR", sock)
			t.Setenv("TMUX", "")
			t.Setenv("GTMUX_LANG", "en")
			restoreResumeFlag = "type"
			t.Cleanup(func() { restoreResumeFlag = ""; _ = exec.Command("tmux", "kill-server").Run(); os.RemoveAll(sock) })
			start := func(name string, args ...string) {
				argv := append([]string{"-f", "/dev/null", "new-session", "-d", "-s", name, "-c", home, "-x", "120", "-y", "20"}, args...)
				if out, err := exec.Command("tmux", argv...).CombinedOutput(); err != nil {
					t.Fatalf("private server: %v %s", err, out)
				}
			}
			start("worker")
			if tc.live {
				start("dev", "sleep 60")
				for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
					if cmd := tmux.Display("dev:0.0", "#{pane_current_command}"); cmd != "" && !isShellCommand(cmd) {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("private live-owner program did not start")
					}
				}
			}
			// Assert isolation before calling the production writer.
			lines := tmux.Lines("list-panes", "-a", "-F", "#{session_name}")
			wantPanes := 1
			if tc.live {
				wantPanes = 2
			}
			if len(lines) != wantPanes {
				t.Fatalf("refusing to type: not isolated: %v", lines)
			}
			for _, line := range lines {
				if line != "worker" && line != "dev" {
					t.Fatalf("foreign session %q", line)
				}
			}
			save := filepath.Join(home, ".tmux", "resurrect", "last")
			if err := os.MkdirAll(filepath.Dir(save), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(save, []byte(savePaneCmd("worker", "0", "0", home, "codex", ":codex")+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			record := func(loc, id string) {
				if err := resume.Save(loc, resume.Record{Agent: "codex", SessionID: id, Cwd: home, UpdatedAt: time.Now().Unix()}); err != nil {
					t.Fatal(err)
				}
			}
			record("old:0.0", "thread-a")
			if tc.live {
				record("dev:0.0", "thread-a")
			}
			if tc.exact {
				record("worker:0.0", "thread-a")
			}
			if tc.ambiguous {
				record("other:0.0", "thread-b")
			}
			resumeAgents()
			var screen string
			for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(50 * time.Millisecond) {
				screen, _ = tmux.Run("capture-pane", "-J", "-p", "-t", "worker:0.0")
				if strings.Contains(screen, "resume") || time.Now().After(deadline) {
					break
				}
			}
			if tc.want == "" && strings.Contains(screen, "resume") {
				t.Fatalf("restore injected a conversation: %q", screen)
			}
			if tc.want != "" && !strings.Contains(screen, "codex resume '"+tc.want+"'") {
				t.Fatalf("unique rename failed: %q", screen)
			}
		})
	}
}
