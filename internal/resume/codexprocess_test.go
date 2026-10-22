package resume

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func processBindingFixture(t *testing.T) (native.Record, codexterminal.Pane, int64) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	now := time.Now().Unix()
	pane := codexterminal.Pane{ID: "%7", Command: "codex", PID: 101, TTY: "/dev/ttys001", Cwd: "/work", Loc: "worker:0.0"}
	r := native.Record{Agent: "codex", HomeID: transcript.CodexHomeID(), SessionID: "new-session", Cwd: pane.Cwd, State: "idle", Event: "Stop", UpdatedAt: now - 10, PID: 202, Comm: "codex", ProcessStart: "UTC-birth"}
	writeProcessSession(t, r.SessionID, "codex-tui", `"cli"`, pane.Cwd)
	writeProcessSession(t, "old-session", "codex-tui", `"cli"`, pane.Cwd)
	if err := native.Save(r); err != nil {
		t.Fatal(err)
	}
	r, _ = native.Load(r.SessionID, r.HomeID)
	if err := Save(pane.Loc, Record{Agent: "codex", HomeID: r.HomeID, SessionID: "old-session", Cwd: pane.Cwd, UpdatedAt: now - 100}); err != nil {
		t.Fatal(err)
	}
	oldPanes, oldClients, oldIdentity := codexProcessPanes, codexProcessClients, codexProcessIdentity
	t.Cleanup(func() {
		codexProcessPanes, codexProcessClients, codexProcessIdentity = oldPanes, oldClients, oldIdentity
	})
	codexProcessPanes = func() []codexterminal.Pane { return []codexterminal.Pane{pane} }
	codexProcessClients = func([]codexterminal.Pane) map[string]codexterminal.Client {
		return map[string]codexterminal.Client{pane.ID: {PID: r.PID, Start: "local-birth", RootStart: "root-birth", TTY: "ttys001"}}
	}
	codexProcessIdentity = func(pid int) (launchhost.AgentProcess, bool) {
		return launchhost.AgentProcess{PID: pid, Comm: "codex", Start: r.ProcessStart, ObservedAt: now}, true
	}
	return r, pane, now
}

func writeProcessSession(t *testing.T, sid, origin, source, cwd string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "22")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]any{"id": sid, "originator": origin, "source": json.RawMessage(source), "cwd": cwd}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-test-"+sid+".jsonl"), append(append(body, '\n'), []byte(`{"type":"event_msg","payload":{"type":"user_message","message":"synthetic task"}}`+"\n")...), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestProcessRecoveryReplacesOnlyExactOldBindingAndPreservesHistory(t *testing.T) {
	r, pane, now := processBindingFixture(t)
	ReconcileCodexProcesses(transcript.NewCodexPathScope(), []native.Record{r}, map[string]string{pane.ID: pane.Loc}, now)
	got, _ := Load(pane.Loc)
	if got.SessionID != r.SessionID || got.HomeID != r.HomeID || got.Cwd != pane.Cwd {
		t.Fatalf("wrong binding: %+v", got)
	}
	if current, ok := native.Load(r.SessionID, r.HomeID); !ok || current != r {
		t.Fatal("native history was changed")
	}
	codexProcessPanes = func() []codexterminal.Pane {
		t.Fatal("already-bound observation repeated process discovery")
		return nil
	}
	ReconcileCodexProcesses(transcript.NewCodexPathScope(), []native.Record{r}, map[string]string{pane.ID: pane.Loc}, now)
}

func TestProcessRecoveryRefusesUnprovenOrChangedOwnership(t *testing.T) {
	for _, kind := range []string{"other home", "wrong birth", "unknown process", "headless or piped", "wrong PID", "missing birth", "missing tty", "multiple panes", "wrong cwd", "other pane location", "newer incumbent", "changed incumbent", "changed observation", "changed pane", "desktop creation", "subagent creation", "missing source", "session end", "multiple sessions", "busy lease", "command stub"} {
		t.Run(kind, func(t *testing.T) {
			r, pane, now := processBindingFixture(t)
			records := []native.Record{r}
			locations := map[string]string{pane.ID: pane.Loc}
			switch kind {
			case "other home":
				r.HomeID = "another-home"
				records = []native.Record{r}
			case "wrong birth":
				codexProcessIdentity = func(int) (launchhost.AgentProcess, bool) {
					return launchhost.AgentProcess{PID: 202, Comm: "codex", Start: "reused-birth"}, true
				}
			case "unknown process":
				codexProcessIdentity = func(int) (launchhost.AgentProcess, bool) { return launchhost.AgentProcess{}, false }
			case "headless or piped":
				codexProcessClients = func([]codexterminal.Pane) map[string]codexterminal.Client { return nil }
			case "wrong PID", "missing birth", "missing tty":
				codexProcessClients = func([]codexterminal.Pane) map[string]codexterminal.Client {
					v := codexterminal.Client{PID: 202, Start: "local-birth", RootStart: "root-birth", TTY: "ttys001"}
					if kind == "wrong PID" {
						v.PID = 303
					}
					if kind == "missing birth" {
						v.RootStart = ""
					}
					if kind == "missing tty" {
						v.TTY = ""
					}
					return map[string]codexterminal.Client{pane.ID: v}
				}
			case "multiple panes":
				peer := pane
				peer.ID = "%8"
				peer.Loc = "peer:0.0"
				codexProcessPanes = func() []codexterminal.Pane { return []codexterminal.Pane{pane, peer} }
				codexProcessClients = func([]codexterminal.Pane) map[string]codexterminal.Client {
					v := codexterminal.Client{PID: 202, Start: "local-birth", RootStart: "root-birth", TTY: "ttys001"}
					return map[string]codexterminal.Client{pane.ID: v, peer.ID: v}
				}
			case "wrong cwd":
				writeProcessSession(t, r.SessionID, "codex-tui", `"cli"`, "/other")
			case "other pane location":
				locations[pane.ID] = "moved:0.0"
			case "newer incumbent":
				if err := Save(pane.Loc, Record{Agent: "codex", SessionID: "old-session", UpdatedAt: now}); err != nil {
					t.Fatal(err)
				}
			case "changed incumbent", "changed observation", "changed pane":
				calls := 0
				codexProcessPanes = func() []codexterminal.Pane {
					calls++
					if calls == 2 {
						switch kind {
						case "changed incumbent":
							_ = Save(pane.Loc, Record{Agent: "codex", SessionID: "newer-owner", UpdatedAt: now})
						case "changed observation":
							_ = native.Save(r)
						case "changed pane":
							p := pane
							p.PID = 404
							return []codexterminal.Pane{p}
						}
					}
					return []codexterminal.Pane{pane}
				}
			case "desktop creation":
				writeProcessSession(t, r.SessionID, "Codex Desktop", `"cli"`, pane.Cwd)
			case "subagent creation":
				writeProcessSession(t, r.SessionID, "codex-tui", `{"subagent":{"parent_thread_id":"parent"}}`, pane.Cwd)
			case "missing source":
				writeProcessSession(t, r.SessionID, "codex-tui", `null`, pane.Cwd)
			case "session end":
				r.Event = "SessionEnd"
				records = []native.Record{r}
			case "multiple sessions":
				peer := r
				peer.SessionID = "another-session"
				writeProcessSession(t, peer.SessionID, "codex-tui", `"cli"`, pane.Cwd)
				records = append(records, peer)
			case "command stub":
				path := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "22", "rollout-test-"+r.SessionID+".jsonl")
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(bytes.SplitN(body, []byte{'\n'}, 2)[0], '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			case "busy lease":
				if err := os.WriteFile(fileFor(pane.Loc)+".binding-lock", nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			ReconcileCodexProcesses(transcript.NewCodexPathScope(), records, locations, now)
			got, _ := Load(pane.Loc)
			if got.SessionID == r.SessionID {
				t.Fatalf("unproven %s acquired binding: %+v", kind, got)
			}
		})
	}
}

func TestFreshMainHookBindsItsOwnVerifiedTerminal(t *testing.T) {
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd", "PostToolUse"} {
		t.Run(event, func(t *testing.T) {
			r, pane, now := processBindingFixture(t)
			process := launchhost.AgentProcess{PID: r.PID, Comm: r.Comm, Start: r.ProcessStart, ObservedAt: now}
			got := BindCodexHookProcess(r.SessionID, r.Cwd, event, process, now)
			want := event == "SessionStart" || event == "UserPromptSubmit"
			if (got == pane.ID) != want {
				t.Fatalf("event %s returned %q", event, got)
			}
		})
	}
}
