package diag

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// A command's actor comes from what the process can see: a delegating component's
// variable, HQ's home, a pane whose agent holds the turn. Anything else is the person at
// the keyboard.
func TestCallerNamesWhoStartedTheCommand(t *testing.T) {
	setup(t, day1)
	t.Setenv(ActorEnv, "")
	t.Setenv("TMUX_PANE", "")
	t.Chdir(t.TempDir())
	if got := Caller(); got != "user" {
		t.Fatalf("a command typed in a terminal: %q, want user", got)
	}

	t.Setenv("TMUX_PANE", "%7")
	if got := Caller(); got != "user" {
		t.Errorf("a pane with no agent mid-turn: %q, want user", got)
	}
	if err := os.MkdirAll(state.ActiveDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state.ActivePath("%7"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Caller(); got != "agent:%7" {
		t.Errorf("a pane whose agent holds the turn: %q, want agent:%%7", got)
	}

	notes := filepath.Join(state.HQHome(), "notes")
	if err := os.MkdirAll(notes, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{state.HQHome(), notes} {
		t.Chdir(dir)
		if got := Caller(); got != "hq" {
			t.Errorf("run from %s: %q, want hq", dir, got)
		}
	}

	t.Setenv(ActorEnv, "menubar")
	if got := Caller(); got != "menubar" {
		t.Errorf("run by the menu bar: %q, want menubar", got)
	}
	t.Setenv(ActorEnv, "phone:evil")
	if got := Caller(); got != "hq" {
		t.Errorf("an actor outside the closed set must be ignored: %q", got)
	}
}

// serve, the hook and the tunnel client are not commands: what they do on their own is
// the system's, whatever pane or directory the process shares.
func TestALongRunningComponentActsAsTheSystem(t *testing.T) {
	dir := setup(t, day1)
	t.Setenv("TMUX_PANE", "")
	SetProcess("serve", "system")
	t.Cleanup(func() { SetProcess("", "") })
	Did("act.knowledge", "pitfalls/x", OK, "changed the knowledge base")
	es := readEntries(t, DayFile(dir, "2026-09-19"))
	if len(es) != 1 || es[0].Component != "serve" || es[0].Actor != "system" {
		t.Fatalf("entries %+v, want one by system under serve", es)
	}
}

// As credits the device only on the goroutine running the verb. serve's ticks run beside
// its handlers, and a tick's record must not be credited to a phone that happened to be
// acting at the time.
func TestAsNamesTheDeviceOnlyWhereTheVerbRuns(t *testing.T) {
	setup(t, day1)
	SetProcess("serve", "system")
	t.Cleanup(func() { SetProcess("", "") })
	inside, beside := "", ""
	var wg sync.WaitGroup
	_ = As("phone:3f9c20e1", func() error {
		inside = Caller()
		wg.Add(1)
		go func() { defer wg.Done(); beside = Caller() }()
		wg.Wait()
		return nil
	})
	if inside != "phone:3f9c20e1" || beside != "system" {
		t.Fatalf("inside the verb %q, on another goroutine %q; want the phone, then system", inside, beside)
	}
	if got := Caller(); got != "system" {
		t.Errorf("after the verb: %q, want system", got)
	}
}

// `debug` in config.json reaches the processes no shell variable does.
func TestDebugFromConfig(t *testing.T) {
	dir := setup(t, day1)
	old := configDebug
	t.Cleanup(func() { configDebug = old })
	configDebug = func() string { return "hook" }
	For("hook").Debug("hook.trace", "shown")
	For("serve").Debug("serve.trace", "hidden")
	configDebug = func() string { return "all" }
	For("serve").Debug("serve.trace2", "shown")
	b, _ := os.ReadFile(DayFile(dir, "2026-09-19"))
	for ev, want := range map[string]bool{"hook.trace": true, "serve.trace": false, "serve.trace2": true} {
		if got := strings.Contains(string(b), `"event":"`+ev+`"`); got != want {
			t.Errorf("%s written=%v, want %v", ev, got, want)
		}
	}
}

// A number read back from the store is a float64; a byte count must still print whole.
func TestAWholeNumberPrintsWhole(t *testing.T) {
	e := Entry{TS: "2026-09-20T00:14:48.000+08:00", Component: "hygiene", Kind: KindAct,
		Event: "act.cleanup", Actor: "system", Target: "hq-feed/spool.jsonl", Outcome: OK,
		Attrs: map[string]any{"bytes": float64(5495510), "ratio": 0.5}}
	got := Format(e, false, false)
	if !strings.Contains(got, "bytes=5495510") || !strings.Contains(got, "ratio=0.5") {
		t.Fatalf("formatted %q", got)
	}
}
