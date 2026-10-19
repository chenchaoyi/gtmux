package hook

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
)

func TestLaunchHostCapturedBeforeDetachWithPayloadEvent(t *testing.T) {
	hermeticEnv(t)
	oldObserve, oldStart := observeLaunchHost, startHookWorker
	defer func() { observeLaunchHost, startHookWorker = oldObserve, oldStart }()
	e := launchhost.Evidence{Name: "sample-client", Kind: "process", ObservedAt: time.Now().Unix(), AgentPID: 11, AgentStart: "born", HostPID: 13, HostStart: "born"}
	var calls []string
	observeLaunchHost = func(commands []string) launchhost.Evidence {
		calls = append(calls, "observe")
		if !slices.Contains(commands, "codex") {
			t.Fatal(commands)
		}
		return e
	}
	raw := `{"hook_event_name":"UserPromptSubmit","session_id":"one","cwd":"/test","prompt":"hello"}`
	startHookWorker = func(args []string, payload []byte, host launchhost.Evidence, process launchhost.AgentProcess) bool {
		calls = append(calls, "detach")
		if host != e || string(payload) != raw {
			t.Fatal(host, string(payload))
		}
		return true
	}
	if Run(strings.NewReader(raw), []string{"--agent", "codex"}) != 0 || !slices.Equal(calls, []string{"observe", "detach"}) {
		t.Fatal(calls)
	}
}
func TestLaunchHostOnlyIdentityHooksObserve(t *testing.T) {
	old := observeLaunchHost
	defer func() { observeLaunchHost = old }()
	calls := 0
	observeLaunchHost = func([]string) launchhost.Evidence { calls++; return launchhost.Evidence{} }
	for _, e := range []string{"Stop", "PostToolUse", "Waiting", "SessionEnd"} {
		hookLaunchHost("codex", e, false)
	}
	if calls != 0 {
		t.Fatal(calls)
	}
	for _, e := range []string{"SessionStart", "UserPromptSubmit", "Resumed"} {
		hookLaunchHost("codex", e, false)
	}
	if calls != 3 {
		t.Fatal(calls)
	}
}
func TestDetachedNativeHostPersistsHistoricallyAndFreshUnknownClears(t *testing.T) {
	hermeticEnv(t)
	e := launchhost.Evidence{Name: "sample-client", Kind: "process", ObservedAt: time.Now().Unix(), AgentPID: 11, AgentStart: "born", HostPID: 13, HostStart: "born"}
	t.Setenv(launchHostEnv, launchhost.Encode(e))
	run := func(event string) native.Record {
		t.Helper()
		raw := `{"session_id":"host-thread","cwd":"/test"}`
		if Run(strings.NewReader(raw), []string{"--agent", "codex", "--detached", event}) != 0 {
			t.Fatal(event)
		}
		records := native.Live(time.Now().Unix())
		if len(records) != 1 {
			t.Fatal(records)
		}
		return records[0]
	}
	if r := run("SessionStart"); r.LaunchHost != e {
		t.Fatal(r.LaunchHost)
	}
	t.Setenv(launchHostEnv, "")
	if r := run("Stop"); r.LaunchHost != e {
		t.Fatal("lost historical evidence", r.LaunchHost)
	}
	if r := run("UserPromptSubmit"); r.LaunchHost != (launchhost.Evidence{}) {
		t.Fatal("stale host retained", r.LaunchHost)
	}
}

func TestDetachedWorkerReceivesFreshHostEnvironment(t *testing.T) {
	out := filepath.Join(t.TempDir(), "host")
	t.Setenv("GTMUX_TEST_HOST_OUT", out)
	t.Setenv(launchHostEnv, "stale")
	if !spawnDetached("/bin/sh", []string{"-c", `printf '%s' "$GTMUX_HOOK_LAUNCH_HOST" > "$GTMUX_TEST_HOST_OUT"`}, nil, launchHostEnv+"=fresh") {
		t.Fatal("worker did not start")
	}
	for range 100 {
		if b, err := os.ReadFile(out); err == nil && len(b) > 0 {
			if string(b) != "fresh" {
				t.Fatalf("inherited stale evidence: %q", b)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("worker did not publish environment evidence")
}
