package agentclient

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func lifecycleFixture(t *testing.T) (Attachment, time.Time) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	return Attachment{Version: ProtocolVersion, Agent: "dsh", Client: "dst", Pane: "%99117", PID: 99117,
		Instance: strings.Repeat("a", 32), HomeID: "test-home", SessionID: "test-session", Cwd: "/test",
		ServerEpoch: "test-server", BindingEpoch: now.Add(-time.Minute).UnixMilli(), StartedAt: now.Add(-time.Minute).UnixMilli(), NativeSession: true}, now
}

func TestLifecycleNativePhasesAndLateCallbacks(t *testing.T) {
	a, now := lifecycleFixture(t)
	emit := func(event, turn string, offset int) Lifecycle {
		t.Helper()
		at := now.Add(time.Duration(offset) * time.Second)
		if err := ObserveLifecycle(a, event, turn, at); err != nil {
			t.Fatal(err)
		}
		v, ok := ReadLifecycle(a, at)
		if !ok {
			t.Fatal("native record missing", event)
		}
		return v
	}
	if v := emit("SessionStart", "", 0); v.Status() != "running" {
		t.Fatal(v)
	}
	v := emit("UserPromptSubmit", "one", 1)
	if v.Status() != "working" {
		t.Fatal(v)
	}
	if v2 := emit("TurnStarted", "one", 2); v2.Since != v.Since {
		t.Fatal("same turn re-dated")
	}
	v = emit("Waiting", "one", 3)
	if v.Status() != "waiting" {
		t.Fatal(v)
	}
	if v2 := emit("SessionStart", "", 4); v2 != v {
		t.Fatal("reconnect reset native phase")
	}
	if v := emit("Resumed", "one", 5); v.Status() != "working" {
		t.Fatal(v)
	}
	v = emit("Interrupt", "one", 6)
	if v.Status() != "idle" || v.EndReason() != "interrupted" {
		t.Fatal(v)
	}
	for i, e := range []string{"Resumed", "Waiting", "Stop", "TurnStarted"} {
		if v2 := emit(e, "one", i+7); v2 != v {
			t.Fatal("late callback changed outcome", e, v2)
		}
	}
	v = emit("TurnStarted", "two", 11)
	if v.Status() != "working" || v.EndReason() != "" {
		t.Fatal(v)
	}
	if v2 := emit("Stop", "one", 12); v2 != v {
		t.Fatal("old turn ended current turn")
	}
	if v := emit("StopFailure", "two", 13); v.EndReason() != "error" {
		t.Fatal(v)
	}
	emit("TurnStarted", "three", 14)
	v = emit("Stop", "three", 15)
	if v.Status() != "idle" || v.EndReason() != "" {
		t.Fatal(v)
	}
	if v2 := emit("Stop", "three", 16); v2.Since != v.Since {
		t.Fatal("duplicate completion re-dated")
	}
	if v := emit("SessionEnd", "", 17); v.Status() != "running" {
		t.Fatal(v)
	}
}

func TestLifecycleExactWitnessAndBoundedOwnedRecord(t *testing.T) {
	for _, kind := range []string{"session", "home", "client", "pane", "pid", "instance", "server", "foreground", "incarnation", "cwd", "future", "pre-incarnation", "unknown event", "invalid version", "long turn", "link", "writable", "oversized", "extra json", "generic markers"} {
		t.Run(kind, func(t *testing.T) {
			a, now := lifecycleFixture(t)
			if err := ObserveLifecycle(a, "Stop", "one", now); err != nil {
				t.Fatal(err)
			}
			path := state.TerminalLifecyclePath(a.Pane)
			var v Lifecycle
			if err := ReadOwnedJSON(path, &v, 16384); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "session":
				a.SessionID = "another"
			case "home":
				a.HomeID = "another"
			case "client":
				a.Client = "another"
			case "pane":
				a.Pane = "%8"
			case "pid":
				a.PID++
			case "instance":
				a.Instance = strings.Repeat("b", 32)
			case "server":
				a.ServerEpoch = "another"
			case "foreground":
				a.BindingEpoch++
			case "incarnation":
				a.StartedAt++
			case "cwd":
				a.Cwd = "/other"
			case "future":
				v.ObservedAt = now.Add(5 * time.Second).UnixMilli()
			case "pre-incarnation":
				v.Since = a.StartedAt - 1
			case "unknown event":
				v.Event = "MadeUp"
			case "invalid version":
				v.Version++
			case "long turn":
				v.TurnID = strings.Repeat("x", 257)
			case "link":
				b, _ := os.ReadFile(path)
				target := filepath.Join(t.TempDir(), "target")
				_ = os.WriteFile(target, b, 0600)
				_ = os.Remove(path)
				_ = os.Symlink(target, path)
			case "writable":
				_ = os.Chmod(path, 0666)
			case "oversized":
				_ = os.WriteFile(path, []byte(strings.Repeat(" ", 16385)), 0600)
			case "extra json":
				b, _ := os.ReadFile(path)
				_ = os.WriteFile(path, append(b, []byte("{}")...), 0600)
			case "generic markers":
				_ = os.Remove(path)
				_ = state.Touch(state.FinishedPath(a.Pane))
				_ = state.WriteMarker(state.TurnOutcomePath(a.Pane), "error")
			}
			if kind == "future" || kind == "pre-incarnation" || kind == "unknown event" || kind == "invalid version" || kind == "long turn" {
				b, _ := json.Marshal(v)
				_ = os.WriteFile(path, b, 0600)
			}
			if _, ok := ReadLifecycle(a, now); ok {
				t.Fatal("untrusted evidence accepted", kind)
			}
		})
	}
}

func TestLifecycleOlderAttachmentCannotOverwriteAndOrphanGC(t *testing.T) {
	a, now := lifecycleFixture(t)
	older := a
	a.StartedAt += 1000
	a.BindingEpoch += 1000
	a.Instance = strings.Repeat("b", 32)
	if err := ObserveLifecycle(a, "Waiting", "new", now); err != nil {
		t.Fatal(err)
	}
	if err := ObserveLifecycle(older, "Stop", "old", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if v, ok := ReadLifecycle(a, now.Add(time.Second)); !ok || v.Status() != "waiting" {
		t.Fatal("old incarnation won", v)
	}
	temp := filepath.Join(state.TerminalLifecycleDir(), ".gtmux-in-flight")
	_ = os.WriteFile(temp, []byte("publishing"), 0600)
	state.ReapOrphanTurnMarkers(map[string]bool{a.Pane: true})
	if _, ok := ReadLifecycle(a, now); !ok {
		t.Fatal("live snapshot reaped")
	}
	state.ReapOrphanTurnMarkers(map[string]bool{})
	if _, err := os.Stat(state.TerminalLifecyclePath(a.Pane)); !os.IsNotExist(err) {
		t.Fatal("orphan retained")
	}
	if _, err := os.Stat(temp); err != nil {
		t.Fatal("atomic publisher temporary file reaped")
	}
}
