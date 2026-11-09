package radar

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestClosedBoundResumedWorkerHistoryAndPreservation(t *testing.T) {
	for _, name := range []string{"completed", "aborted", "same explicit turn", "different turn", "new observation", "new submit", "waiting", "new start", "live frontend", "no receipt", "legacy binding", "nonshared", "future completion", "ambiguous completion", "other home", "explicit follow"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			now := time.Now().Unix()
			process, ok := launchhost.ReadAgent(os.Getpid())
			if !ok {
				t.Fatal("cannot inspect isolated test process")
			}
			child := exec.Command("true")
			if err := child.Run(); err != nil {
				t.Fatal(err)
			}
			if syscall.Kill(child.Process.Pid, 0) != syscall.ESRCH {
				t.Skip("exited child PID has already been reused; cannot establish closed frontend")
			}
			r := native.Record{Agent: "codex", HomeID: transcript.CodexHomeID(), SessionID: "worker", Cwd: t.TempDir(), State: "working", Event: "Resumed", PID: process.PID, Comm: process.Comm, ProcessStart: process.Start, UpdatedAt: now - 5}
			kind, at := "task_complete", now-11
			switch name {
			case "aborted":
				kind = "turn_aborted"
			case "same explicit turn":
				r.TurnID = "old-turn"
			case "different turn":
				r.TurnID = "new-turn"
			case "new submit":
				r.Event = "UserPromptSubmit"
			case "waiting":
				r.Event, r.State = "Waiting", "waiting"
			case "new start":
				kind, at = "task_started", now-1
			case "future completion":
				at = now + 60
			case "other home":
				r.HomeID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			}
			if err := native.Save(r); err != nil {
				t.Fatal(err)
			}
			r, _ = native.Load(r.SessionID, r.HomeID)
			b := native.Binding{Kind: "frontend", HomeID: r.HomeID, SessionID: r.SessionID, Cwd: r.Cwd, ID: "binding", Pane: "%999", Loc: "synthetic:0.0", PID: child.Process.Pid, PanePID: child.Process.Pid, BackendPID: r.PID, BackendStart: r.ProcessStart, PreparedAt: now - 30, BoundAt: now - 1}
			if name == "live frontend" {
				b.PID = os.Getppid() // definite live process distinct from backend
			}
			if name == "legacy binding" {
				b.Kind = ""
			}
			if name != "no receipt" {
				if err := native.ConfirmBinding(b, r); err != nil {
					t.Fatal(err)
				}
			}
			if name == "new observation" {
				if err := native.Save(r); err != nil {
					t.Fatal(err)
				}
				r, _ = native.Load(r.SessionID, r.HomeID)
			}
			dir := filepath.Join(home, "sessions", "2026", "11", "09")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			body := func(kind string) []byte {
				return []byte(`{"type":"session_meta","payload":{"id":"worker","originator":"codex-tui"}}` + "\n" +
					`{"timestamp":"` + time.Unix(at, 0).UTC().Format(time.RFC3339) + `","type":"event_msg","payload":{"type":"` + kind + `","turn_id":"old-turn"}}` + "\n")
			}
			path := filepath.Join(dir, "rollout-a-worker.jsonl")
			if err := os.WriteFile(path, body(kind), 0600); err != nil {
				t.Fatal(err)
			}
			if name == "ambiguous completion" {
				if err := os.WriteFile(filepath.Join(dir, "rollout-b-worker.jsonl"), body("task_started"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "explicit follow" {
				if _, err := sessionpolicy.Save("worker", sessionpolicy.Settings{HQ: true}); err != nil {
					t.Fatal(err)
				}
			}
			old := sharedCodexBackend
			probes := 0
			sharedCodexBackend = func(v native.Record) bool {
				probes++
				if v.PID != process.PID || v.ProcessStart != process.Start {
					t.Fatal("role probe changed process identity")
				}
				return name != "nonshared"
			}
			t.Cleanup(func() { sharedCodexBackend = old })
			rows := nativePanes(nil, nil, now)
			wantHidden := name == "completed" || name == "aborted" || name == "same explicit turn"
			if (len(rows) == 0) != wantHidden {
				t.Fatalf("rows=%+v want hidden=%v", rows, wantHidden)
			}
			if len(rows) == 1 {
				want := r.State
				if name == "explicit follow" {
					want = "idle"
				}
				if rows[0].Status != want || rows[0].source != "native" || rows[0].adoptable || rows[0].currentClient != "unconfirmed" {
					t.Fatalf("state/control changed: %+v, want %s", rows[0], want)
				}
				if name == "explicit follow" && (rows[0].Follow == nil || !rows[0].Follow.HQ || rows[0].Follow.Knowledge || rows[0].Follow.Notify) {
					t.Fatal("follow authority changed", rows[0])
				}
				encoded, err := agentsJSONBytes(rows)
				if err != nil {
					t.Fatal(err)
				}
				var wire []map[string]any
				if err := json.Unmarshal(encoded, &wire); err != nil || len(wire) != 1 || wire[0]["source"] != "native" || wire[0]["current_client"] != "unconfirmed" || wire[0]["status"] != want {
					t.Fatalf("JSON identity/state changed: %s / %v", encoded, err)
				}
			}
			if probes > 1 {
				t.Fatalf("repeated backend role probe: %d", probes)
			}
			if current, ok := native.Load(r.SessionID, r.HomeID); !ok || current != r {
				t.Fatal("presentation removed or rewrote observation", current)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != string(body(kind)) {
				t.Fatal("presentation changed history", err)
			}
		})
	}
}
