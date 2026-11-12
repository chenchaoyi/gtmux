package resume

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func publicationFixture(t *testing.T) (CodexFrontend, CodexBindingIntent, int64) {
	t.Helper()
	f, _ := frontendFixture(t)
	cwd := t.TempDir()
	codexProcessPanes = func() []codexterminal.Pane {
		return []codexterminal.Pane{{ID: f.Pane, Command: "bash", TTY: "/dev/ttys999", Cwd: cwd, Loc: f.Loc, PID: f.PanePID}}
	}
	var err error
	f, err = BeginCodexFrontend(f.Pane, f.Instance, f.PID, f.BackendPID, f.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{"incumbent", "bootstrap-session", "new-root"} {
		writeProcessSession(t, sid, "codex-tui", `"cli"`, cwd)
	}
	now := time.Now().Unix()
	if err := Save(f.Loc, Record{Agent: "codex", HomeID: f.HomeID, SessionID: "incumbent", Cwd: cwd, UpdatedAt: now - 100}); err != nil {
		t.Fatal(err)
	}
	wire, err := PrepareCodexBinding(f.CodexBindingTarget, "synthetic publication task", now)
	if err != nil {
		t.Fatal(err)
	}
	intent, ok := CodexBindingForPrompt(wire, now)
	if !ok {
		t.Fatal("fixture intent did not validate")
	}
	return f, intent, now
}

func assertPublicationUnlocked(t *testing.T) {
	t.Helper()
	l, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		t.Fatalf("publication leaked shared lease: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCBP001ActualBootstrapCannotOverwriteNewFrontend(t *testing.T) {
	for _, sid := range []string{"new-root", "bootstrap-session"} {
		t.Run(sid, func(t *testing.T) {
			f, intent, now := publicationFixture(t)
			checked, resumeOld, done := make(chan Record, 1), make(chan struct{}), make(chan error, 1)
			original := codexBindingCheckpoint
			codexBindingCheckpoint = func(r, previous Record) Record {
				checked <- previous
				<-resumeOld
				return original(r, previous)
			}
			go func() { done <- CompleteCodexBinding(intent, f.CodexBindingTarget, "bootstrap-session", now) }()
			released := false
			defer func() {
				if !released {
					close(resumeOld)
				}
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("bootstrap did not finish")
				}
				codexBindingCheckpoint = original
			}()
			select {
			case previous := <-checked:
				if previous != intent.Incumbent {
					t.Fatal("barrier did not follow the actual incumbent check")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("bootstrap did not reach checkpoint I/O")
			}
			newer, err := BindCodexFrontend(f, sid)
			if err != nil {
				t.Fatalf("new frontend could not publish during old preparation: %v", err)
			}
			primary, _ := os.ReadFile(fileFor(f.Loc))
			receipt, _ := os.ReadFile(frontendPath(f.Instance))
			close(resumeOld)
			released = true
			var oldErr error
			select {
			case oldErr = <-done:
				done <- oldErr
			case <-time.After(3 * time.Second):
				t.Fatal("publication deadlocked")
			}
			if oldErr == nil {
				t.Error("old bootstrap published after a new frontend acquired the location")
			}
			after, _ := os.ReadFile(fileFor(f.Loc))
			if string(after) != string(primary) {
				t.Error("old bootstrap overwrote the new primary index")
			}
			after, _ = os.ReadFile(frontendPath(f.Instance))
			if string(after) != string(receipt) {
				t.Error("old bootstrap changed the new frontend receipt")
			}
			if _, err := os.Stat(codexBindingPath(intent.Token)); err != nil {
				t.Error("rejected bootstrap consumed its retry witness")
			}
			if got, ok := LoadLive(f.Loc); !ok || got.SessionID != sid || got.Frontend != f.Instance {
				t.Error("new owner/session/frontend are inconsistent")
			}
			assertPublicationUnlocked(t)
			if _, err := RefreshCodexFrontend(newer); err != nil {
				t.Errorf("new owner's heartbeat failed: %v", err)
			}
			if err := CompleteCodexBinding(intent, f.CodexBindingTarget, "bootstrap-session", now); sid != "bootstrap-session" && err == nil {
				t.Error("retry reclaimed a different new owner")
			}
		})
	}
}

func TestCBP002ActualPublishersRespectSharedLease(t *testing.T) {
	t.Run("bootstrap", func(t *testing.T) {
		target, _, intent, now := bindingFixture(t, "codex-tui")
		lease, err := state.TryLease("codex-frontend-binding")
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
		if err := CompleteCodexBinding(intent, target, "worker-session", now); !errors.Is(err, state.ErrLeaseHeld) {
			t.Fatalf("bootstrap ignored the shared lease: %v", err)
		}
		if _, ok := Load(target.Loc); ok {
			t.Fatal("contended bootstrap changed the index")
		}
		if _, err := os.Stat(codexBindingPath(intent.Token)); err != nil {
			t.Fatal("contention consumed retry witness")
		}
		lease.Close()
		if err := CompleteCodexBinding(intent, target, "worker-session", now); err != nil {
			t.Fatal("uncontended retry failed", err)
		}
		assertPublicationUnlocked(t)
	})
	t.Run("process reconcile", func(t *testing.T) {
		r, pane, now := processBindingFixture(t)
		before, _ := os.ReadFile(fileFor(pane.Loc))
		lease, err := state.TryLease("codex-frontend-binding")
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
		ReconcileCodexProcesses(transcript.NewCodexPathScope(), []native.Record{r}, map[string]string{pane.ID: pane.Loc}, now)
		after, _ := os.ReadFile(fileFor(pane.Loc))
		if string(after) != string(before) {
			t.Fatal("process publisher ignored the shared lease")
		}
		if got, ok := native.Load(r.SessionID, r.HomeID); !ok || got != r {
			t.Fatal("deferred process publication changed its observation")
		}
		lease.Close()
		ReconcileCodexProcesses(transcript.NewCodexPathScope(), []native.Record{r}, map[string]string{pane.ID: pane.Loc}, now)
		if got, ok := Load(pane.Loc); !ok || got.SessionID != r.SessionID {
			t.Fatal("process retry did not publish")
		}
		assertPublicationUnlocked(t)
	})
}

func TestCBP003FailurePathsReleasePublicationLocks(t *testing.T) {
	for _, kind := range []string{"changed intent", "legacy lock", "index write", "receipt write"} {
		t.Run(kind, func(t *testing.T) {
			target, _, intent, now := bindingFixture(t, "codex-tui")
			var blocker string
			switch kind {
			case "changed intent":
				if err := os.WriteFile(codexBindingPath(intent.Token), []byte(`{}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "legacy lock":
				blocker = fileFor(target.Loc) + ".binding-lock"
				if err := os.WriteFile(blocker, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "index write":
				blocker = fileFor(target.Loc)
				if err := os.Mkdir(blocker, 0700); err != nil {
					t.Fatal(err)
				}
			case "receipt write":
				if err := native.Save(native.Record{Agent: "codex", HomeID: transcript.CodexHomeID(), SessionID: "worker-session", Cwd: target.Cwd, State: "idle", Event: "SessionStart", UpdatedAt: now}); err != nil {
					t.Fatal(err)
				}
				blocker = filepath.Join(native.Dir(), "bindings")
				if err := os.WriteFile(blocker, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(codexBindingPath(intent.Token))
			err := CompleteCodexBinding(intent, target, "worker-session", now)
			if err == nil {
				t.Fatal("fault was reported as success")
			}
			if kind == "receipt write" {
				var receiptErr *CodexBindingReceiptError
				if !errors.As(err, &receiptErr) {
					t.Fatal("committed publication lost receipt error type", err)
				}
			}
			after, _ := os.ReadFile(codexBindingPath(intent.Token))
			if string(after) != string(before) {
				t.Fatal("failure changed retry evidence")
			}
			assertPublicationUnlocked(t)
			if kind != "legacy lock" {
				if _, err := os.Stat(fileFor(target.Loc) + ".binding-lock"); !os.IsNotExist(err) {
					t.Fatal("failure leaked location lock")
				}
			}
			if blocker != "" {
				if err := os.Remove(blocker); err != nil {
					t.Fatal(err)
				}
				if err := CompleteCodexBinding(intent, target, "worker-session", now); err != nil {
					t.Fatal("retry after fault failed", err)
				}
			}
		})
	}
}

func TestCBP003SameOwnerProcessPreservesFrontend(t *testing.T) {
	f, _, now := publicationFixture(t)
	f, err := BindCodexFrontend(f, "bootstrap-session")
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(f.Loc, Record{Agent: "codex", HomeID: f.HomeID, SessionID: f.SessionID, Cwd: f.Cwd, TerminalClient: "codex", UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	oldClients, oldIdentity, oldInteractive := codexProcessClients, codexProcessIdentity, codexProcessInteractive
	t.Cleanup(func() {
		codexProcessClients, codexProcessIdentity, codexProcessInteractive = oldClients, oldIdentity, oldInteractive
	})
	codexProcessClients = frontendClients
	codexProcessIdentity = func(pid int) (launchhost.AgentProcess, bool) {
		return launchhost.AgentProcess{PID: pid, Comm: "codex", Start: "UTC-client-birth", ObservedAt: now}, true
	}
	codexProcessInteractive = func(int) bool { return true }
	if got := BindCodexHookProcess(f.SessionID, f.Cwd, "UserPromptSubmit", launchhost.AgentProcess{PID: f.PID, Comm: "codex", Start: "UTC-client-birth", ObservedAt: now}, now); got != f.Pane {
		t.Fatal("exact process failed to bind")
	}
	if got, ok := LoadLive(f.Loc); !ok || got.Frontend != f.Instance || got.SessionID != f.SessionID || got.TerminalClient != "codex" {
		t.Fatal("same-owner process publication lost attachment identity")
	}
	if _, err := RefreshCodexFrontend(f); err != nil {
		t.Fatal("same-owner heartbeat failed", err)
	}
}

type publicationChildInput struct {
	Intent CodexBindingIntent
	Target CodexBindingTarget
	Now    int64
	Held   bool
	Home   string
}

func TestCBP004Child(t *testing.T) {
	path := os.Getenv("GTMUX_CBP_CHILD")
	if path == "" {
		t.Skip("subprocess-only entry")
	}
	if !filepath.IsAbs(path) || !strings.Contains(path, "TestCBP004CrossProcess") {
		t.Fatal("refuse non-fixture input")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var in publicationChildInput
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("HOME") != in.Home || !strings.Contains(state.Dir(), "TestCBP004CrossProcess") {
		t.Fatal("refuse non-fixture state")
	}
	err = CompleteCodexBinding(in.Intent, in.Target, "worker-session", in.Now)
	if in.Held {
		if !errors.Is(err, state.ErrLeaseHeld) {
			t.Fatalf("child publisher bypassed parent lease: %v", err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
}

func TestCBP004CrossProcessPublication(t *testing.T) {
	target, _, intent, now := bindingFixture(t, "codex-tui")
	path := filepath.Join(t.TempDir(), "child-input.json")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func(held bool) {
		t.Helper()
		b, err := json.Marshal(publicationChildInput{intent, target, now, held, os.Getenv("HOME")})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestCBP004Child$", "-test.count=1")
		cmd.Env = append(os.Environ(), "GTMUX_CBP_CHILD="+path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture child failed: %v\n%s", err, out)
		}
	}
	lease, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	run(true)
	if _, ok := Load(target.Loc); ok {
		t.Fatal("child wrote while parent held lease")
	}
	if _, err := os.Stat(codexBindingPath(intent.Token)); err != nil {
		t.Fatal("child consumed intent while contended")
	}
	lease.Close()
	run(false)
	if got, ok := Load(target.Loc); !ok || got.SessionID != "worker-session" {
		t.Fatal("uncontended child did not publish")
	}
	assertPublicationUnlocked(t)
}
