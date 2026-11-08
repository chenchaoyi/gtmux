package hook

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func completedTurnHookFixture(t *testing.T) func(string, string, string) {
	t.Helper()
	hermeticEnv(t)
	t.Setenv("CODEX_HOME", "")
	old := codexHookFrontend
	t.Cleanup(func() { codexHookFrontend = old })
	codexHookFrontend = func(home, sid string) (resume.CodexFrontend, bool) {
		return resume.CodexFrontend{BackendPID: 200, BackendStart: "synthetic-birth", SessionID: sid, CodexBindingTarget: resume.CodexBindingTarget{Pane: "%71"}}, home == transcript.CodexHomeID()
	}
	t.Setenv(agentProcessEnv, launchhost.EncodeAgent(launchhost.AgentProcess{PID: 200, Comm: "codex", Start: "synthetic-birth", ObservedAt: time.Now().Unix()}))
	return func(event, sid, turn string) {
		b, err := json.Marshal(map[string]string{"session_id": sid, "turn_id": turn, "prompt": "synthetic next task", "trigger": "auto"})
		if err != nil {
			t.Fatal(err)
		}
		if code := Run(strings.NewReader(string(b)), []string{"--agent", "codex", "--detached", event}); code != 0 {
			t.Fatal(code)
		}
	}
}

func TestCompletedCodexTurnRejectsExplicitLateRearm(t *testing.T) {
	// CET-004: production Run, real completion and turn marker, no clock sleeps.
	for _, event := range []string{"PostToolUse", "PostCompact"} {
		t.Run(event, func(t *testing.T) {
			run := completedTurnHookFixture(t)
			run("UserPromptSubmit", "thread", "one")
			run("Stop", "thread", "one")
			before, ok := state.ReadCompletion("%71")
			if !ok {
				t.Fatal("missing Stop completion")
			}
			records, _ := events.ReadSince(0)
			run(event, "thread", "one")
			after, ok := state.ReadCompletion("%71")
			if state.Exists(state.ActivePath("%71")) || !ok || after != before {
				t.Fatal("late callback resurrected completed turn or erased completion")
			}
			afterRecords, _ := events.ReadSince(0)
			if len(records) != len(afterRecords) {
				t.Fatal("rejected callback emitted new lifecycle event")
			}
			run("UserPromptSubmit", "thread", "two")
			run(event, "thread", "one")
			if state.ReadMarker(state.TurnIDPath("%71")) != "two" || state.ReadMarker(state.ActivePath("%71")) != "thread" {
				t.Fatal("late old callback changed new turn")
			}
			run("PostToolUse", "thread", "two")
			if !state.Exists(state.ActivePath("%71")) {
				t.Fatal("new ongoing turn was fenced")
			}
		})
	}
}

func TestCompletedCodexTurnKeepsLegacyAndUncertainCallbacks(t *testing.T) {
	// CET-004: incomplete identity is not evidence to suppress a callback.
	for _, tc := range []struct {
		name, sid, turn, completionSID, completionTurn string
		wait                                           bool
	}{
		{name: "missing callback turn", sid: "thread", completionSID: "thread", completionTurn: "one"},
		{name: "other session completion", sid: "thread", turn: "one", completionSID: "other", completionTurn: "one"},
		{name: "legacy completion without turn", sid: "thread", turn: "one", completionSID: "thread"},
		{name: "approval continuation", sid: "thread", turn: "one", completionSID: "thread", completionTurn: "one", wait: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := completedTurnHookFixture(t)
			if err := state.WriteMarker(state.TurnIDPath("%71"), "one"); err != nil {
				t.Fatal(err)
			}
			if err := state.RecordCompletion("%71", "codex", tc.completionSID, tc.completionTurn, time.Now().Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			if tc.wait {
				if err := state.WriteMarker(state.WaitingPath("%71"), "permission"); err != nil {
					t.Fatal(err)
				}
			}
			run("PostToolUse", tc.sid, tc.turn)
			if state.ReadMarker(state.ActivePath("%71")) != "thread" {
				t.Fatal("uncertain or actual approval callback suppressed")
			}
		})
	}
	t.Run("new legacy prompt clears old turn", func(t *testing.T) {
		run := completedTurnHookFixture(t)
		run("UserPromptSubmit", "thread", "one")
		run("Stop", "thread", "one")
		run("UserPromptSubmit", "thread", "")
		run("PostToolUse", "thread", "")
		if !state.Exists(state.ActivePath("%71")) || state.Exists(state.TurnIDPath("%71")) {
			t.Fatal("legacy new turn inherited completed identity")
		}
	})
}

func TestCodexContinuationPreservesActiveStart(t *testing.T) {
	// CET-005: an old mtime makes renewal detection deterministic.
	for _, event := range []string{"PostToolUse", "PostCompact"} {
		t.Run(event, func(t *testing.T) {
			run := completedTurnHookFixture(t)
			run("UserPromptSubmit", "thread", "one")
			start := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
			if err := os.Chtimes(state.ActivePath("%71"), start, start); err != nil {
				t.Fatal(err)
			}
			run(event, "thread", "one")
			fi, err := os.Stat(state.ActivePath("%71"))
			if err != nil || !fi.ModTime().Equal(start) {
				t.Fatal("continuation changed actual start time", fi, err)
			}
			run("UserPromptSubmit", "thread", "two")
			fi, err = os.Stat(state.ActivePath("%71"))
			if err != nil || !fi.ModTime().After(start) {
				t.Fatal("actual next prompt failed to refresh start")
			}
			if err := os.Chtimes(state.ActivePath("%71"), start, start); err != nil {
				t.Fatal(err)
			}
			run("PostToolUse", "new-thread", "two")
			fi, err = os.Stat(state.ActivePath("%71"))
			if err != nil || !fi.ModTime().After(start) || state.ReadMarker(state.ActivePath("%71")) != "new-thread" {
				t.Fatal("different owner failed to refresh marker")
			}
		})
	}
}
