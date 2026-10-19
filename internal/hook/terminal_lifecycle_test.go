package hook

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestNativeTerminalLifecyclePublishedOnlyForVerifiedSender(t *testing.T) {
	for _, kind := range []string{"verified", "wrong pid", "wrong instance", "wrong client", "unverified", "web"} {
		t.Run(kind, func(t *testing.T) {
			hermeticEnv(t)
			t.Setenv("DSH_HOME", t.TempDir())
			t.Setenv("TMUX_PANE", "%99117") // inherited Web pane must never count
			now := time.Now()
			a := agentclient.Attachment{Version: 1, Agent: "dsh", Client: "dst", Pane: "%99117", PID: 99117, Instance: strings.Repeat("a", 32), HomeID: dsh.HomeID(), SessionID: "session", ServerEpoch: "synthetic", StartedAt: now.Add(-time.Minute).UnixMilli(), BindingEpoch: now.Add(-time.Minute).UnixMilli(), NativeSession: true}
			previous, previousTmux := terminalForSession, tmux.Bin
			terminalForSession = func(agent, home, id string) (agentclient.Attachment, bool) {
				return a, kind != "unverified" && agent == a.Agent && home == a.HomeID && id == a.SessionID
			}
			tmux.Bin = ""
			defer func() { terminalForSession, tmux.Bin = previous, previousTmux }()
			payload := map[string]any{"session_id": a.SessionID, "terminal_client": a.Client, "client_pid": a.PID, "client_instance": a.Instance, "turn_id": "one"}
			switch kind {
			case "wrong pid":
				payload["client_pid"] = 99118
			case "wrong instance":
				payload["client_instance"] = strings.Repeat("b", 32)
			case "wrong client":
				payload["terminal_client"] = "another"
			case "web":
				delete(payload, "terminal_client")
			}
			run := func(event string) {
				t.Helper()
				b, _ := json.Marshal(payload)
				if got := Run(strings.NewReader(string(b)), []string{"--agent", "dsh", "--detached", event}); got != 0 {
					t.Fatal(got)
				}
			}
			run("TurnStarted")
			v, ok := agentclient.ReadLifecycle(a, time.Now())
			if ok != (kind == "verified") {
				t.Fatal("sender provenance mismatch", kind, v)
			}
			if kind == "verified" {
				if v.Status() != "working" {
					t.Fatal(v)
				}
				run("Question")
				if v, ok := agentclient.ReadLifecycle(a, time.Now()); !ok || v.Status() != "waiting" {
					t.Fatal("canonical native question not recorded", v)
				}
				run("Resumed")
				run("Stop")
				if v, ok := agentclient.ReadLifecycle(a, time.Now()); !ok || v.Status() != "idle" {
					t.Fatal("native completion not recorded", v)
				}
				payload["turn_id"] = "two"
				run("TurnStarted")
				payload["turn_id"] = "one"
				run("Stop")
				if v, _ := agentclient.ReadLifecycle(a, time.Now()); v.Status() != "working" || v.TurnID != "two" {
					t.Fatal("rejected stale turn wrote evidence", v)
				}
			}
		})
	}
}
