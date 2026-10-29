package resume

import (
	"os"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func frontendFixture(t *testing.T) (CodexFrontend, *codexterminal.Client) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	oldClients, oldProcess, oldPanes, oldView := frontendClients, frontendProcess, codexProcessPanes, frontendPaneView
	t.Cleanup(func() {
		frontendClients, frontendProcess, codexProcessPanes, frontendPaneView = oldClients, oldProcess, oldPanes, oldView
	})
	c := &codexterminal.Client{PID: 101, Start: "client-start", RootStart: "root-start", TTY: "ttys999"}
	frontendClients = func([]codexterminal.Pane) map[string]codexterminal.Client {
		return map[string]codexterminal.Client{"%9": *c}
	}
	codexProcessPanes = func() []codexterminal.Pane {
		return []codexterminal.Pane{{ID: "%9", Command: "bash", TTY: "/dev/ttys999", Cwd: "/synthetic", Loc: "work:0.0", PID: 100}}
	}
	frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
		if pid == os.Getpid() {
			return launchhost.AgentProcess{PID: pid, Comm: "gtmux", Start: "bridge-start"}, true
		}
		if pid == 200 {
			return launchhost.AgentProcess{PID: pid, Comm: "codex", Start: "backend-start"}, true
		}
		return launchhost.AgentProcess{}, false
	}
	frontendPaneView = func(string, string) string { return "%9\t100\twork:0.0" }
	f, err := BeginCodexFrontend("%9", "0123456789abcdef0123456789abcdef", 101, 200, 1)
	if err != nil {
		t.Fatal(err)
	}
	return f, c
}

func TestFrontendBindingSurvivesHooksButNotExit(t *testing.T) {
	f, _ := frontendFixture(t)
	var err error
	f, err = BindCodexFrontend(f, "root-a")
	if err != nil {
		t.Fatal(err)
	}
	r, ok := LoadLive(f.Loc)
	if !ok || r.SessionID != "root-a" || r.Frontend != f.Instance {
		t.Fatal("root did not bind", r, ok)
	}
	if err := Save(f.Loc, Record{Agent: "codex", HomeID: f.HomeID, SessionID: "root-a"}); err != nil {
		t.Fatal(err)
	}
	if saved, _ := Load(f.Loc); saved.Frontend != f.Instance {
		t.Fatal("hook dropped frontend proof")
	}
	if err := Save(f.Loc, Record{Agent: "codex", HomeID: f.HomeID, SessionID: "foreign"}); err == nil {
		t.Fatal("foreign hook stole live frontend")
	}
	EndCodexFrontend(f)
	if _, ok := LoadLive(f.Loc); ok {
		t.Fatal("exited frontend still owns a live pane")
	}
	if saved, ok := Load(f.Loc); !ok || saved.SessionID != "root-a" {
		t.Fatal("exit deleted restore history")
	}
}

func TestFrontendRejectsPeerBirthAndHomeConflicts(t *testing.T) {
	f, client := frontendFixture(t)
	if _, err := BeginCodexFrontend("%9", f.Instance, 999, 200, 1); err == nil {
		t.Fatal("unrelated peer accepted")
	}
	if _, err := BindCodexFrontend(f, "root-a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := CodexFrontendForThread(transcript.CodexHomeID(), "root-a"); !ok {
		t.Fatal("verified root missing")
	}
	if _, ok := CodexFrontendForThread("foreign-home", "root-a"); ok {
		t.Fatal("home scope crossed")
	}
	client.Start = "reused-pid"
	if _, ok := CodexFrontendForThread(f.HomeID, "root-a"); ok {
		t.Fatal("reused client PID retained input authority")
	}
	if _, err := BindCodexFrontend(f, "other"); err == nil {
		t.Fatal("changed client stole next root")
	}
}

func TestNewConnectionPreventsLateOldBinding(t *testing.T) {
	f, _ := frontendFixture(t)
	f, err := BindCodexFrontend(f, "old")
	if err != nil {
		t.Fatal(err)
	}
	next := f
	next.Instance, next.Sequence = "abcdef0123456789abcdef0123456789", 2
	if _, err := BindCodexFrontend(next, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := BindCodexFrontend(f, "late-old"); err == nil {
		t.Fatal("old connection reclaimed new root")
	}
	EndCodexFrontend(f)
	if got, ok := LoadLive(f.Loc); !ok || got.SessionID != "new" {
		t.Fatal("old disconnect removed newer attachment", got)
	}
}

func TestFrontendExpiryKeepsRestoreRecord(t *testing.T) {
	f, _ := frontendFixture(t)
	f, err := BindCodexFrontend(f, "root")
	if err != nil {
		t.Fatal(err)
	}
	f.StartedAt, f.UpdatedAt = time.Now().Add(-time.Minute).UnixMilli(), time.Now().Add(-6*time.Second).UnixMilli()
	if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadLive(f.Loc); ok {
		t.Fatal("expired bridge retained live ownership")
	}
	if _, ok := Load(f.Loc); !ok {
		t.Fatal("expiry removed history")
	}
}

func TestFrontendRetiresOnlyItsOwnBackendStartup(t *testing.T) {
	for _, event := range []string{"SessionStart", "UserPromptSubmit"} {
		t.Run(event, func(t *testing.T) {
			f, _ := frontendFixture(t)
			r := native.Record{Agent: "codex", HomeID: f.HomeID, SessionID: "root", Cwd: f.Cwd, PID: 200, ProcessStart: "backend-start", Comm: "codex", Event: event, State: "idle", UpdatedAt: time.Now().Unix()}
			if err := native.Save(r); err != nil {
				t.Fatal(err)
			}
			if _, err := BindCodexFrontend(f, "root"); err != nil {
				t.Fatal(err)
			}
			_, exists := native.Load("root", f.HomeID)
			if exists != (event != "SessionStart") {
				t.Fatal("wrong native generation retired", event, exists)
			}
		})
	}
}

func TestFrontendRetainsThreadWorkspaceSeparateFromPhysicalPane(t *testing.T) {
	f, _ := frontendFixture(t)
	f, err := BindCodexFrontend(f, "saved-root", "/saved/project")
	if err != nil {
		t.Fatal(err)
	}
	r, ok := LoadLive(f.Loc)
	if !ok || r.Cwd != "/saved/project" || f.Cwd != "/synthetic" || f.ThreadCwd != r.Cwd {
		t.Fatal("thread and terminal workspace conflated", f, r)
	}
	if _, ok := CodexFrontendForThread(f.HomeID, "saved-root"); !ok {
		t.Fatal("workspace difference lost root authority")
	}
}

func TestFrontendRepairsOnlySameThreadOldHookPublication(t *testing.T) {
	f, _ := frontendFixture(t)
	f, err := BindCodexFrontend(f, "root", "/saved/project")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := Load(f.Loc)
	r.Frontend = ""
	if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
		t.Fatal(err)
	}
	f, err = RefreshCodexFrontend(f)
	if err != nil {
		t.Fatal("old hook dropped verified attachment", err)
	}
	if live, ok := LoadLive(f.Loc); !ok || live.Frontend != f.Instance {
		t.Fatal("same-thread old hook lost identity", live)
	}
	r.SessionID = "other"
	if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshCodexFrontend(f); err == nil {
		t.Fatal("heartbeat stole foreign thread")
	}
}
