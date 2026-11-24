package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const claudeForegroundFixture = "GTMUX_TEST_HQ_CLAUDE_FOREGROUND"

func TestHQClaudeHookFollowsForegroundShellChild(t *testing.T) {
	switch os.Getenv(claudeForegroundFixture) {
	case "hook":
		payload, err := json.Marshal(map[string]string{
			"hook_event_name": "SessionStart",
			"session_id":      "hq-successor-fixture",
			"cwd":             state.HQHome(),
		})
		if err != nil {
			t.Fatal(err)
		}
		Run(strings.NewReader(string(payload)), []string{"--agent", "claude", "SessionStart"})
		return
	case "client":
		input := bufio.NewReader(os.Stdin)
		fmt.Println("fixture-client-ready")
		if _, err := input.ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run", "^TestHQClaudeHookFollowsForegroundShellChild$")
		cmd.Env = append(os.Environ(), claudeForegroundFixture+"=hook")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture hook: %v\n%s", err, out)
		}
		fmt.Println("fixture-hook-returned")
		_, _ = input.ReadString('\n')
		return
	case "replacement":
		fmt.Println("fixture-client-ready")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		return
	}

	if tmux.Bin == "" {
		t.Skip("tmux unavailable")
	}
	dir, err := os.MkdirTemp("/tmp", "gtmux-hq-client-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("HOME", dir)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TMUX_TMPDIR", dir)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("BASH_ENV", "")
	if err := os.MkdirAll(state.HQHome(), 0700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	clientPath := filepath.Join(dir, "claude")
	version := filepath.Join(dir, "2.1.123")
	if err := os.Link(executable, version); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(version, clientPath); err != nil {
		t.Fatal(err)
	}
	pane, err := tmux.Run("-f", "/dev/null", "new-session", "-d", "-P", "-F", "#{pane_id}",
		"-s", "hq-client-proof", "-c", state.HQHome(), "/bin/bash", "--noprofile", "--norc", "-i")
	if err != nil || pane == "" {
		t.Fatalf("start private tmux shell: pane %q, %v", pane, err)
	}
	t.Cleanup(func() { _ = tmux.OK("kill-server") })
	root := tmux.Display(pane, "#{pane_pid}")
	generation := tmux.PaneGeneration(pane)
	loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
	wait := func(description string, condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for !condition() {
			if !time.Now().Before(deadline) {
				t.Fatalf("waiting for %s\n%s", description, tmux.CapturePane(pane))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	launch := func(role string) {
		t.Helper()
		command := claudeForegroundFixture + "=" + role + " " + clientPath +
			" -test.run '^TestHQClaudeHookFollowsForegroundShellChild$'"
		if err := tmux.SendText(pane, command, true); err != nil {
			t.Fatal(err)
		}
	}
	wait("interactive shell", func() bool { return tmux.Display(pane, "#{pane_current_command}") == "bash" })
	launch("client")
	wait("fixture client to own the foreground", func() bool {
		command := tmux.Display(pane, "#{pane_current_command}")
		return (command == "claude" || command == "2.1.123") &&
			strings.Contains(tmux.CapturePane(pane), "fixture-client-ready")
	})
	if err := tmux.SendText(pane, "fixture-reset", true); err != nil {
		t.Fatal(err)
	}
	wait("fixture hook to return", func() bool { return strings.Contains(tmux.CapturePane(pane), "fixture-hook-returned") })
	rec, ok := resume.Load(loc)
	if !ok || rec.SessionID != "hq-successor-fixture" {
		t.Fatalf("hook did not save successor: %+v", rec)
	}
	w := rec.HQObservation
	tty := strings.TrimPrefix(tmux.Display(pane, "#{pane_tty}"), "/dev/")
	if generation == "" || w.Generation != generation || w.ClientPID <= 1 || root == fmt.Sprint(w.ClientPID) ||
		w.ClientBirth == "" || w.ClientTTY == "" || w.ClientTTY != tty || w.ObservedAtNS <= 0 {
		t.Fatalf("hook did not witness the actual foreground shell child: root=%s witness=%+v", root, w)
	}
	if !resume.MatchesLegacyHQClient(pane, rec) {
		t.Fatal("live shell child did not confirm its own saved hook witness")
	}
	if _, ok := codexterminal.ForegroundClaude(pane, os.Getpid()); ok {
		t.Fatal("an unrelated process qualified as the foreground client's hook")
	}
	for _, alter := range []func(*resume.HQObservation){
		func(w *resume.HQObservation) { w.Generation += "-obsolete" },
		func(w *resume.HQObservation) { w.ClientBirth += "-obsolete" },
		func(w *resume.HQObservation) { w.ClientTTY += "-other" },
	} {
		changed := rec
		alter(&changed.HQObservation)
		if resume.MatchesLegacyHQClient(pane, changed) {
			t.Fatalf("mismatched witness still confirms: %+v", changed.HQObservation)
		}
	}
	if !tmux.OK("send-keys", "-t", pane, "C-z") {
		t.Fatal("could not suspend private foreground job")
	}
	wait("shell to regain foreground while the original client still exists", func() bool {
		return tmux.Display(pane, "#{pane_current_command}") == "bash" &&
			strings.Contains(tmux.CapturePane(pane), "Stopped")
	})
	if resume.MatchesLegacyHQClient(pane, rec) {
		t.Fatal("stopped client still confirms")
	}
	launch("replacement")
	wait("replacement Claude child to take the same pane", func() bool {
		client, ok := codexterminal.ForegroundClaude(pane, 0)
		return ok && client.PID != w.ClientPID
	})
	if tmux.PaneGeneration(pane) != generation || resume.MatchesLegacyHQClient(pane, rec) {
		t.Fatal("same generation replacement accepted the old client witness")
	}
}
