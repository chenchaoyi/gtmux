package hook

import (
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
	"strings"
	"testing"
)

func TestTerminalOutcomePayloads(t *testing.T) {
	for _, tc := range []struct{ agent, event, payload, want string }{
		{"codex", "Interrupt", `{}`, "Interrupt"},
		{"kimi", "Interrupt", `{"reason":"user"}`, "Interrupt"},
		{"kimi", "TurnStarted", `{"origin_kind":"task"}`, "TurnStarted"},
		{"cursor", "Stop", `{"status":"aborted"}`, "Interrupt"},
		{"cursor", "Stop", `{"status":"error"}`, "StopFailure"},
		{"cursor", "Stop", `{"status":"completed"}`, "Stop"},
		{"cursor", "Stop", `{}`, ""},
		{"cursor", "Stop", `{"status":"new-upstream-status"}`, ""},
		{"copilot", "ErrorOccurred", `{"recoverable":false}`, "StopFailure"},
		{"copilot", "ErrorOccurred", `{"recoverable":true}`, ""},
		{"copilot", "ErrorOccurred", `{}`, ""},
		{"gemini", "Notification", `{"notification_type":"ToolPermission"}`, "Waiting"},
		{"gemini", "Notification", `{"notification_type":"warning"}`, ""},
		{"claude", "Elicitation", `{}`, "Waiting"},
		{"claude", "ElicitationResult", `{}`, "Resumed"},
		{"claude", "PostToolUseFailure", `{}`, "Resumed"},
		{"kimi", "PostToolUseFailure", `{}`, ""},
	} {
		t.Run(tc.agent+tc.event+tc.payload, func(t *testing.T) {
			if got := classifyPayload(tc.agent, tc.event, "Bash", []byte(tc.payload)); got.Lifecycle != tc.want {
				t.Fatalf("got %+v; want %s", got, tc.want)
			}
		})
	}
}

func TestInterruptedTurnClearsStateWithoutFinishingAndLateToolCannotReviveIt(t *testing.T) {
	hermeticEnv(t)
	const pane = "%71"
	t.Setenv("TMUX_PANE", pane)
	_ = state.WriteMarker(state.ActivePath(pane), "session")
	_ = state.WriteMarker(state.WaitingPath(pane), "permission")
	_ = state.WriteMarker(state.TurnIDPath(pane), "turn-1")
	_ = state.WriteLastFinished("%other")
	Run(strings.NewReader(`{"session_id":"session","turn_id":"turn-1"}`), []string{"--agent", "kimi", "Interrupt"})
	if state.Exists(state.ActivePath(pane)) || state.Exists(state.WaitingPath(pane)) || state.Exists(state.FinishedPath(pane)) {
		t.Fatal("interrupted turn retained busy state or earned finished stamp")
	}
	if state.ReadMarker(state.TurnOutcomePath(pane)) != "interrupted" {
		t.Fatal("poller lost interruption outcome")
	}
	if state.ReadLastFinished() != "%other" {
		t.Fatal("interruption changed latest completed pane")
	}
	if nativeState, _ := nativeStateFor("Interrupt"); nativeState != "idle" {
		t.Fatal("native interruption is not idle")
	}
	Run(strings.NewReader(`{"session_id":"session","turn_id":"turn-1"}`), []string{"--agent", "kimi", "PermissionResult"})
	if state.Exists(state.ActivePath(pane)) {
		t.Fatal("late tool callback revived interrupted turn")
	}
	Run(strings.NewReader(`{"session_id":"session","turn_id":"turn-2","prompt":"new task"}`), []string{"--agent", "kimi", "UserPromptSubmit"})
	if !state.Exists(state.ActivePath(pane)) || state.Exists(state.TurnOutcomePath(pane)) {
		t.Fatal("new actual prompt did not restore working state")
	}
	Run(strings.NewReader(`{"session_id":"session","turn_id":"turn-1"}`), []string{"--agent", "kimi", "Interrupt"})
	if !state.Exists(state.ActivePath(pane)) {
		t.Fatal("old interruption ended new turn")
	}
}

func TestNativeInterruptionPersistsOutcome(t *testing.T) {
	hermeticEnv(t)
	Run(strings.NewReader(`{"session_id":"native-kimi","cwd":"/work"}`), []string{"--agent", "kimi", "Interrupt"})
	r, ok := native.Load("native-kimi")
	if !ok || r.State != "idle" || r.EndReason != "interrupted" {
		t.Fatalf("record: %+v", r)
	}
}

func TestToolFailureIsNotTerminalFailure(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("TMUX_PANE", "%3")
	_ = state.WriteMarker(state.ActivePath("%3"), "kimi")
	Run(strings.NewReader(`{"session_id":"kimi","error":"tool exit 2"}`), []string{"--agent", "kimi", "PostToolUseFailure"})
	if !state.Exists(state.ActivePath("%3")) || state.Exists(state.TurnOutcomePath("%3")) {
		t.Fatal("tool failure ended the parent turn")
	}
}

func TestNativeLateCallbacksCannotReviveOrEndAnotherTurn(t *testing.T) {
	hermeticEnv(t)
	run := func(event, id string) {
		Run(strings.NewReader(`{"session_id":"native","turn_id":"`+id+`","prompt":"work"}`), []string{"--agent", "kimi", event})
	}
	run("TurnStarted", "one")
	run("Interrupt", "one")
	run("PermissionResult", "one")
	if r, _ := native.Load("native"); r.State != "idle" || r.EndReason != "interrupted" {
		t.Fatal(r)
	}
	run("TurnStarted", "two")
	run("Interrupt", "one")
	if r, _ := native.Load("native"); r.State != "working" || r.TurnID != "two" {
		t.Fatal(r)
	}
}

func TestSessionResetClearsOldTurnIdentityButCompactionPreservesIt(t *testing.T) {
	hermeticEnv(t)
	const pane = "%71"
	t.Setenv("TMUX_PANE", pane)
	_ = state.WriteMarker(state.ActivePath(pane), "old-session")
	_ = state.WriteMarker(state.TurnIDPath(pane), "old-turn")
	Run(strings.NewReader(`{"session_id":"old-session"}`), []string{"--agent", "kimi", "SessionStart"})
	if state.ReadMarker(state.TurnIDPath(pane)) != "old-turn" {
		t.Fatal("same-session continuation lost turn identity")
	}
	Run(strings.NewReader(`{"session_id":"new-session"}`), []string{"--agent", "kimi", "SessionStart"})
	if state.Exists(state.TurnIDPath(pane)) {
		t.Fatal("new session inherited old turn identity")
	}
	Run(strings.NewReader(`{"session_id":"new-session","turn_id":"new-turn"}`), []string{"--agent", "kimi", "PermissionRequest"})
	if !state.Exists(state.WaitingPath(pane)) {
		t.Fatal("old turn identity suppressed new startup approval")
	}
	_ = state.WriteMarker(state.ActivePath(pane), "new-session")
	Run(strings.NewReader(`{"session_id":"old-session"}`), []string{"--agent", "kimi", "SessionEnd"})
	if state.ReadMarker(state.ActivePath(pane)) != "new-session" {
		t.Fatal("old session end cleared new session")
	}
}
