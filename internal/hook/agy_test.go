package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestAgyClassifyPayload(t *testing.T) {
	cases := []struct {
		name    string
		event   string
		payload string
		want    Class
	}{
		{"session start", "SessionStart", `{"conversationId":"c1"}`, Class{Lifecycle: "SessionStart"}},
		{"first invocation opens turn", "PreInvocation", `{"invocationNum":1,"lastUserInput":"fix tests"}`, Class{Lifecycle: "UserPromptSubmit"}},
		{"zero invocation opens turn", "PreInvocation", `{"lastUserInput":"fix tests"}`, Class{Lifecycle: "UserPromptSubmit"}},
		{"subsequent invocation resumes turn", "PreInvocation", `{"invocationNum":3,"lastUserInput":"fix tests"}`, Class{Lifecycle: "Resumed"}},
		{"post tool clears wait", "PostToolUse", `{"toolCall":{"name":"run_command"}}`, Class{Lifecycle: "Resumed"}},
		{"pre tool stays telemetry", "PreToolUse", `{"toolCall":{"name":"run_command"}}`, Class{}},
		{"stop normal", "Stop", `{"terminationReason":"NO_TOOL_CALL"}`, Class{Lifecycle: "Stop"}},
		{"stop terminal step", "Stop", `{"terminationReason":"TERMINAL_STEP_TYPE"}`, Class{Lifecycle: "Stop"}},
		{"stop user canceled", "Stop", `{"terminationReason":"USER_CANCELED"}`, Class{Lifecycle: "Interrupt"}},
		{"stop error", "Stop", `{"terminationReason":"ERROR","error":"timeout"}`, Class{Lifecycle: "StopFailure"}},
		{"stop max invocations", "Stop", `{"terminationReason":"MAX_INVOCATIONS"}`, Class{Lifecycle: "StopFailure"}},
		{"stop token budget", "Stop", `{"terminationReason":"MAX_TOKEN_BUDGET_EXCEEDED"}`, Class{Lifecycle: "StopFailure"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyPayload("agy", tc.event, "", []byte(tc.payload)); got != tc.want {
				t.Fatalf("classifyPayload(agy, %q) = %+v, want %+v", tc.event, got, tc.want)
			}
		})
	}
}

func TestAgyHookRunEmitsContractJSONAndRecordsState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TMUX_PANE", "%91")

	origStdout := agyHookStdout
	origStart := startHookWorker
	defer func() {
		agyHookStdout = origStdout
		startHookWorker = origStart
	}()

	// Verify sync detach triggers for non-detached agy hooks and stays off for --detached.
	if !shouldDetachSyncHook("agy", []string{"--agent", "agy", "Stop"}) {
		t.Fatal("non-detached agy hook should detach sync work")
	}
	if shouldDetachSyncHook("agy", []string{"--detached", "--agent", "agy", "Stop"}) {
		t.Fatal("--detached agy worker must not re-detach")
	}

	// Force synchronous execution in-process by returning false from startHookWorker so we
	// verify both stdout JSON contract and state mutations in one run.
	startHookWorker = func([]string, []byte, launchhost.Evidence, launchhost.AgentProcess) bool { return false }

	var buf bytes.Buffer
	agyHookStdout = &buf
	prePayload := `{"conversationId":"agy-sess-1","cwd":"/tmp/work","modelName":"gemini-4-neon-high","invocationNum":1,"hookEventName":"PreInvocation","lastUserInput":"<USER_REQUEST>\nRefactor parser\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nnow\n</ADDITIONAL_METADATA>"}`
	if rc := Run(strings.NewReader(prePayload), []string{"--agent", "agy", "PreInvocation"}); rc != 0 {
		t.Fatalf("PreInvocation rc = %d, want 0", rc)
	}
	if got := strings.TrimSpace(buf.String()); got != `{}` {
		t.Fatalf("PreInvocation stdout = %q, want {}", got)
	}
	if !state.Exists(state.ActivePath("%91")) {
		t.Fatal("PreInvocation(1) did not mark pane active")
	}
	if got := cleanAgyPromptText("<USER_REQUEST>\nRefactor parser\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nnow\n</ADDITIONAL_METADATA>"); got != "Refactor parser" {
		t.Fatalf("cleanAgyPromptText = %q", got)
	}

	// Stop emits required decision JSON so agy allows the loop to terminate cleanly.
	buf.Reset()
	stopPayload := `{"conversationId":"agy-sess-1","cwd":"/tmp/work","terminationReason":"NO_TOOL_CALL","fullyIdle":true}`
	if rc := Run(strings.NewReader(stopPayload), []string{"--agent", "agy", "Stop"}); rc != 0 {
		t.Fatalf("Stop rc = %d, want 0", rc)
	}
	var stopResp map[string]string
	if err := json.Unmarshal(buf.Bytes(), &stopResp); err != nil || stopResp["decision"] != "stop" {
		t.Fatalf("Stop stdout = %q, want JSON with decision=stop", buf.String())
	}
	if state.Exists(state.ActivePath("%91")) {
		t.Fatal("Stop did not clear active marker")
	}

	// User cancellation marks turn outcome as interrupted rather than finished.
	buf.Reset()
	cancelPayload := `{"conversationId":"agy-sess-1","cwd":"/tmp/work","terminationReason":"USER_CANCELED"}`
	if rc := Run(strings.NewReader(cancelPayload), []string{"--agent", "agy", "Stop"}); rc != 0 {
		t.Fatalf("Cancel Stop rc = %d, want 0", rc)
	}
	if got := state.ReadMarker(state.TurnOutcomePath("%91")); got != "interrupted" {
		t.Fatalf("TurnOutcome = %q, want interrupted", got)
	}

	// Detached worker must NOT print stdout JSON again.
	buf.Reset()
	if rc := Run(strings.NewReader(stopPayload), []string{"--detached", "--agent", "agy", "Stop"}); rc != 0 {
		t.Fatalf("detached Stop rc = %d", rc)
	}
	if buf.Len() != 0 {
		t.Fatalf("detached worker wrote stdout %q, want empty", buf.String())
	}
}

func TestAgySubagentHookIgnoredWithoutMutatingParent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TMUX_PANE", "%92")

	origStdout := agyHookStdout
	origStart := startHookWorker
	defer func() {
		agyHookStdout = origStdout
		startHookWorker = origStart
	}()
	startHookWorker = func([]string, []byte, launchhost.Evidence, launchhost.AgentProcess) bool { return false }

	var buf bytes.Buffer
	agyHookStdout = &buf

	// Start parent turn on %92.
	parentPayload := `{"conversationId":"parent-conv","rootConversationId":"parent-conv","cwd":"/tmp/work","invocationNum":1,"hookEventName":"PreInvocation","lastUserInput":"Parent task"}`
	if rc := Run(strings.NewReader(parentPayload), []string{"--agent", "agy", "PreInvocation"}); rc != 0 {
		t.Fatalf("parent PreInvocation rc = %d", rc)
	}
	if got := state.ReadMarker(state.ActivePath("%92")); got != "parent-conv" {
		t.Fatalf("active marker = %q, want parent-conv", got)
	}

	// Subagent Stop carrying rootConversationId != conversationId must still emit valid JSON
	// stdout ({"decision":"stop"}) while leaving parent active marker untouched.
	buf.Reset()
	subStopPayload := `{"conversationId":"child-subagent","rootConversationId":"parent-conv","cwd":"/tmp/work","terminationReason":"NO_TOOL_CALL","hookEventName":"Stop"}`
	if rc := Run(strings.NewReader(subStopPayload), []string{"--agent", "agy", "Stop"}); rc != 0 {
		t.Fatalf("subagent Stop rc = %d", rc)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"decision":"stop"}` {
		t.Fatalf("subagent Stop stdout = %q, want decision=stop JSON", got)
	}
	if got := state.ReadMarker(state.ActivePath("%92")); got != "parent-conv" {
		t.Fatalf("subagent Stop clobbered parent active marker: %q", got)
	}
}
