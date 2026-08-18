package hook

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// hermeticEnv isolates each test's filesystem + tmux/terminal coupling:
//   - HOME → temp dir so state markers don't touch the real ~/.local/share/gtmux.
//   - TMUX_PANE unset by default so Run() takes the no-tmux, state-less path
//     (no tmux/osascript shell-outs). Tests that want a pane set it explicitly.
func hermeticEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMUX_PANE", "")
	t.Setenv("GTMUX_HOOK_DEBUG", "")
	// Clearing $TMUX_PANE is no longer enough to mean "not in tmux": a hook with no
	// pane env now asks the PROCESS TREE which pane it belongs to (paneidentity.go),
	// and `go test` run inside a tmux pane has exactly that ancestry — so a test
	// asserting the no-pane path would silently exercise the pane path instead.
	// Pointing tmux at an empty socket dir leaves it with no server to answer from.
	// $TMUX names the live server's socket and OVERRIDES $TMUX_TMPDIR, so both go.
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", t.TempDir())
}

// TestRunAlwaysZero: a hook must never fail the agent's turn, so Run returns 0
// for every input — known events, unknown agents, unmapped events, and garbage.
func TestRunAlwaysZero(t *testing.T) {
	cases := []struct {
		name  string
		stdin string
		args  []string
	}{
		{"claude stop via stdin", `{"hook_event_name":"Stop"}`, nil},
		{"claude prompt via stdin", `{"hook_event_name":"UserPromptSubmit"}`, nil},
		{"claude notification via stdin", `{"hook_event_name":"Notification"}`, nil},
		{"unknown agent", `{"hook_event_name":"Stop"}`, []string{"--agent", "nope"}},
		{"unmapped event", `{"hook_event_name":"Frobnicate"}`, nil},
		{"codex positional event", "", []string{"--agent", "codex", "agent-turn-complete"}},
		{"codex json payload positional", "", []string{"--agent", "codex", `{"type":"agent-turn-complete"}`}},
		{"malformed json stdin", "{not json", nil},
		{"empty stdin no args", "", nil},
		{"--agent with no value", "", []string{"--agent"}},
		{"stray flag is ignored", `{"hook_event_name":"Stop"}`, []string{"--weird"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hermeticEnv(t)
			if got := Run(strings.NewReader(c.stdin), c.args); got != 0 {
				t.Errorf("Run(%q, %v) = %d, want 0", c.stdin, c.args, got)
			}
		})
	}
}

// TestStdinIsTerminal pins the char-device guard: a TTY or /dev/null is a character
// device (never drained — reading a TTY blocks forever and steals the agent's input),
// while a pipe or an in-memory reader carries a real payload and must be drained.
func TestStdinIsTerminal(t *testing.T) {
	// /dev/null is a character device — the guard must report true so Run skips it.
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	defer devnull.Close()
	if !stdinIsTerminal(devnull) {
		t.Error("/dev/null (a character device, like a TTY) must be treated as terminal → not drained")
	}

	// A pipe is NOT a character device — a real payload flows through it, so it is drained.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	if stdinIsTerminal(r) {
		t.Error("a pipe must NOT be treated as terminal — the hook payload arrives this way")
	}

	// An in-memory reader is not an *os.File at all → drained.
	if stdinIsTerminal(strings.NewReader("payload")) {
		t.Error("a non-*os.File reader must not be treated as terminal")
	}
}

// TestRunDoesNotBlockOnTTYStdin is the opencode-plugin-wedge regression: when a plugin
// runs `gtmux hook` without redirecting stdin, the hook inherits the agent's controlling
// TTY. io.ReadAll on a TTY never EOFs, so a pre-fix Run would hang here (holding the
// pane's input). We stand in a character device (/dev/null) for the TTY and assert Run
// returns promptly instead of blocking.
func TestRunDoesNotBlockOnTTYStdin(t *testing.T) {
	hermeticEnv(t)
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	defer devnull.Close()

	done := make(chan int, 1)
	go func() { done <- Run(devnull, []string{"--agent", "opencode", "Stop"}) }()
	select {
	case got := <-done:
		if got != 0 {
			t.Errorf("Run over a char-device stdin = %d, want 0", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run blocked on a character-device stdin — the TTY-drain guard regressed")
	}
}

// TestRunNoPaneNoStateWrites: with no TMUX_PANE, Run can't key state, so it must
// not create any active/waiting/last-finished markers (it degrades to a
// state-less notify path). We assert the state dir stays clean for a Stop event.
func TestRunNoPaneNoStateWrites(t *testing.T) {
	hermeticEnv(t)
	Run(strings.NewReader(`{"hook_event_name":"Stop"}`), nil)

	if state.ReadLastFinished() != "" {
		t.Error("no-pane Stop must not write last-finished")
	}
	// No pane key exists, so nothing under active/ or waiting/ should be created.
	if got := state.WaitingSet(); len(got) != 0 {
		t.Errorf("no-pane run should leave waiting empty, got %v", got)
	}
}

// TestRunUnknownEventNoStateWrites: an unmapped event short-circuits before any
// state mutation even when a pane is present.
func TestRunUnknownEventNoStateWrites(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("TMUX_PANE", "%5")
	Run(strings.NewReader(`{"hook_event_name":"Frobnicate"}`), nil)

	if state.Exists(state.ActivePath("%5")) {
		t.Error("unmapped event must not create an active marker")
	}
	if state.ReadLastFinished() != "" {
		t.Error("unmapped event must not write last-finished")
	}
}

// TestRunUnknownAgentNoStateWrites: an unknown --agent is a no-op regardless of
// the event carried.
func TestRunUnknownAgentNoStateWrites(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("TMUX_PANE", "%6")
	Run(strings.NewReader(`{"hook_event_name":"Stop"}`), []string{"--agent", "ghost"})

	if state.Exists(state.ActivePath("%6")) || state.ReadLastFinished() != "" {
		t.Error("unknown agent must mutate no state")
	}
}

// TestRunPositionalEventBeatsStdin: a positional event token wins over stdin's
// hook_event_name (Codex passes the event as an arg; stdin may be empty/other).
func TestRunPositionalEventBeatsStdin(t *testing.T) {
	hermeticEnv(t)
	// Claude agent, but pass a positional "UserPromptSubmit" while stdin says Stop.
	t.Setenv("TMUX_PANE", "%8")
	Run(strings.NewReader(`{"hook_event_name":"Stop"}`), []string{"UserPromptSubmit"})

	// UserPromptSubmit (positional) → active marker on; Stop would have left none.
	if !state.Exists(state.ActivePath("%8")) {
		t.Error("positional UserPromptSubmit should win and set the active marker")
	}
	if state.ReadLastFinished() != "" {
		t.Error("UserPromptSubmit must not write last-finished (Stop would have)")
	}
}

// TestRunDebugLog: with GTMUX_HOOK_DEBUG set, Run writes a trace line to
// <state.Dir>/hook.log. Hermetic via temp HOME.
func TestRunDebugLog(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("GTMUX_HOOK_DEBUG", "1")
	Run(strings.NewReader(`{"hook_event_name":"Stop"}`), nil)

	logPath := state.Dir() + "/hook.log"
	if !state.Exists(logPath) {
		t.Fatalf("expected debug log at %s", logPath)
	}
}

// TestExtractEventMore covers JSON edge cases beyond the existing happy-path test.
func TestExtractEventMore(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  Stop  ", "  Stop  "},         // non-JSON token returned verbatim (whitespace kept)
		{`  {"type":"x"}  `, "x"},        // leading space before { still parsed
		{`{"type":""}`, `{"type":""}`},   // empty type → fall through to raw token
		{`{"type":"a","type":"b"}`, "b"}, // duplicate keys: last wins (encoding/json)
		{"{}", "{}"},                     // no type → raw
		{`{"Type":"x"}`, "x"},            // encoding/json matches field names case-insensitively
		{`{"type":123}`, `{"type":123}`}, // type not a string → unmarshal error → raw
	}
	for _, c := range cases {
		if got := extractEvent(c.in); got != c.want {
			t.Errorf("extractEvent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRunSupersededStopIgnored: a Stop from a session that no longer owns the
// pane (a newer session was started, e.g. after /clear) must NOT clear the
// current session's active marker — it's a late, out-of-order hook (#5908).
func TestRunSupersededStopIgnored(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("TMUX_PANE", "%1")

	// Session A owns the pane.
	if err := state.WriteMarker(state.ActivePath("%1"), "sessA"); err != nil {
		t.Fatal(err)
	}
	// A late Stop tagged with a DIFFERENT session is ignored.
	Run(strings.NewReader(`{"hook_event_name":"Stop","session_id":"sessB"}`), nil)
	if !state.Exists(state.ActivePath("%1")) {
		t.Fatal("superseded Stop cleared the active marker")
	}
	if got := state.ReadMarker(state.ActivePath("%1")); got != "sessA" {
		t.Fatalf("active session = %q, want sessA", got)
	}
	// A Stop tagged with the OWNING session does clear it.
	Run(strings.NewReader(`{"hook_event_name":"Stop","session_id":"sessA"}`), nil)
	if state.Exists(state.ActivePath("%1")) {
		t.Fatal("matching Stop did not clear the active marker")
	}
}

// TestRunClassifierWaiting: the classifier drives waiting + its kind. Generic
// agents escalate side-effecting tools; Claude escalates only plan/question
// (and keeps its Notification-as-permission timing); the waiting marker carries
// the kind.
func TestRunClassifierWaiting(t *testing.T) {
	cases := []struct {
		name      string
		agent     string
		stdin     string
		preActive bool   // pre-create the active marker (mid-turn)
		wantKind  string // expected waiting marker content; "" = no waiting marker
	}{
		{"gemini side-effecting tool → permission", "gemini",
			`{"hook_event_name":"PreToolUse","tool_name":"Bash"}`, false, "permission"},
		{"gemini read-only tool → no waiting", "gemini",
			`{"hook_event_name":"PreToolUse","tool_name":"Read"}`, false, ""},
		{"claude plan tool → plan", "claude",
			`{"hook_event_name":"PreToolUse","tool_name":"ExitPlanMode"}`, false, "plan"},
		{"claude question tool → question", "claude",
			`{"hook_event_name":"PreToolUse","tool_name":"AskUserQuestion"}`, false, "question"},
		{"claude bash pre-tool → no waiting", "claude",
			`{"hook_event_name":"PreToolUse","tool_name":"Bash"}`, false, ""},
		{"claude notification mid-turn → permission", "claude",
			`{"hook_event_name":"Notification"}`, true, "permission"},
		{"claude notification idle → no waiting", "claude",
			`{"hook_event_name":"Notification"}`, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hermeticEnv(t)
			t.Setenv("TMUX_PANE", "%5")
			if c.preActive {
				_ = state.Touch(state.ActivePath("%5"))
			}
			var args []string
			if c.agent != "claude" {
				args = []string{"--agent", c.agent}
			}
			Run(strings.NewReader(c.stdin), args)
			got := ""
			if state.Exists(state.WaitingPath("%5")) {
				if got = state.ReadMarker(state.WaitingPath("%5")); got == "" {
					got = "<empty>"
				}
			}
			if got != c.wantKind {
				t.Fatalf("waiting marker = %q, want %q", got, c.wantKind)
			}
		})
	}
}
