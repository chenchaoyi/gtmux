package hook

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The codex-only, no-re-detach gating for the SYNC-hook detach.
func TestShouldDetachCodexHook(t *testing.T) {
	cases := []struct {
		agent string
		args  []string
		want  bool
	}{
		{"codex", []string{"--agent", "codex", "UserPromptSubmit"}, true},
		{"codex", []string{"--detached", "--agent", "codex", "UserPromptSubmit"}, false}, // worker must not re-detach
		{"claude", []string{"--agent", "claude", "Stop"}, false},                         // non-codex untouched
		{"opencode", []string{"--agent", "opencode", "Stop"}, false},
	}
	for _, c := range cases {
		if got := shouldDetachCodexHook(c.agent, c.args); got != c.want {
			t.Errorf("shouldDetachCodexHook(%q, %v) = %v, want %v", c.agent, c.args, got, c.want)
		}
	}
}

// The payload must reach the detached worker even though the hook process exits right
// after starting it. With a pipe fed by os/exec's copy goroutine it usually did not: the
// goroutine died with the process, and the worker read nothing (on 2026-10-04, 11 of 413
// Codex UserPromptSubmit records carried their prompt). This runs the real race: a child
// test process spawns the worker and exits at once, twenty times, and every payload must
// arrive whole.
func TestDetachedWorkerGetsThePayloadAfterTheHookExits(t *testing.T) {
	if os.Getenv("GTMUX_TEST_DETACH_CHILD") == "1" {
		out := os.Getenv("GTMUX_TEST_DETACH_OUT")
		payload := []byte(os.Getenv("GTMUX_TEST_DETACH_PAYLOAD"))
		if !spawnDetached("/bin/sh", []string{"-c", `cat > "$0.tmp" && mv "$0.tmp" "$0"`, out}, payload) {
			os.Exit(3)
		}
		os.Exit(0) // exit at once, as the hook does
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// Big enough to need more than one pipe write, small enough to stay quick.
	payload := `{"hook_event_name":"UserPromptSubmit","session_id":"019a","prompt":"` + strings.Repeat("检查 ", 30000) + `"}`
	for i := 0; i < 20; i++ {
		out := filepath.Join(dir, fmt.Sprintf("payload-%d", i))
		cmd := exec.Command(exe, "-test.run=^TestDetachedWorkerGetsThePayloadAfterTheHookExits$")
		cmd.Env = append(os.Environ(), "GTMUX_TEST_DETACH_CHILD=1", "GTMUX_TEST_DETACH_OUT="+out,
			"GTMUX_TEST_DETACH_PAYLOAD="+payload)
		if err := cmd.Run(); err != nil {
			t.Fatalf("run %d: the hook could not start its worker: %v", i, err)
		}
		var got []byte
		for wait := 0; wait < 100; wait++ {
			if got, err = os.ReadFile(out); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if string(got) != payload {
			t.Fatalf("run %d: the worker read %d bytes, want %d", i, len(got), len(payload))
		}
	}
}

func TestPayloadFileLeavesNothingOnDisk(t *testing.T) {
	f, err := payloadFile([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := os.Stat(f.Name()); !os.IsNotExist(err) {
		t.Fatalf("the payload file is still on disk: %v", err)
	}
	b, _ := io.ReadAll(f)
	if string(b) != "hello" {
		t.Fatalf("read %q", b)
	}
}
