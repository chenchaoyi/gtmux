package hook

import "testing"

func TestClassify(t *testing.T) {
	w := func(k Kind) Class { return Class{Lifecycle: "Waiting", Kind: k, Actionable: true} }
	tele := Class{}
	resume := Class{Lifecycle: "Resumed"}

	cases := []struct {
		name             string
		source, ev, tool string
		want             Class
	}{
		// Claude's PreToolUse escalates ONLY the always-blocking plan/question
		// tools (#4985 still holds: an ordinary, often auto-approved, tool start
		// is never an approval).
		{"claude pre-tool bash = telemetry", "claude", "PreToolUse", "Bash", tele},
		{"claude pre-tool exitplan = plan", "claude", "PreToolUse", "ExitPlanMode", w(KindPlan)},
		{"claude pre-tool question = question", "claude", "PreToolUse", "AskUserQuestion", w(KindQuestion)},
		{"claude permission bash = permission", "claude", "PermissionRequest", "Bash", w(KindPermission)},
		{"claude permission plan = plan", "claude", "PermissionRequest", "ExitPlanMode", w(KindPlan)},
		{"claude permission question = question", "claude", "PermissionRequest", "AskUserQuestion", w(KindQuestion)},
		{"claude prompt = turn start", "claude", "UserPromptSubmit", "", Class{Lifecycle: "UserPromptSubmit"}},
		{"claude stop = turn end", "claude", "Stop", "", Class{Lifecycle: "Stop"}},
		// PostToolUse (matcher-scoped at install time to plan/question) → the wait
		// is resolved, so clear it.
		{"claude post-tool = resume", "claude", "PostToolUse", "ExitPlanMode", resume},
		{"claude notification = telemetry", "claude", "Notification", "", tele},
		{"claude unknown event = telemetry", "claude", "Frobnicate", "", tele},
		// Session lifecycle → dedicated events (the state machine clears markers).
		{"claude session start", "claude", "SessionStart", "", Class{Lifecycle: "SessionStart"}},
		{"claude session end", "claude", "SessionEnd", "", Class{Lifecycle: "SessionEnd"}},

		// Codex's PreToolUse fires for every tool → telemetry; its PermissionRequest
		// (new hooks system) is a real user-facing approval → waiting.
		{"codex pre-tool bash = telemetry", "codex", "PreToolUse", "Bash", tele},
		{"codex permission = waiting", "codex", "PermissionRequest", "shell", w(KindPermission)},
		{"codex turn complete = stop", "codex", "agent-turn-complete", "", Class{Lifecycle: "Stop"}},

		// Hermes: pre_tool_call telemetry, separate approval event.
		{"hermes pre-tool = telemetry", "hermes-agent", "pre_tool_call", "terminal", tele},
		{"hermes approval = permission", "hermes-agent", "pre_approval_request", "terminal", w(KindPermission)},
		{"hermes approval answered = resume", "hermes-agent", "post_approval_response", "terminal", resume},

		// Generic agents (gemini/cursor/copilot/…): the pre-tool event is the
		// ONLY signal, so side-effecting tools escalate, read-only stay telemetry.
		{"generic pre-tool bash = permission", "gemini", "PreToolUse", "Bash", w(KindPermission)},
		{"generic pre-tool read = telemetry", "gemini", "PreToolUse", "Read", tele},
		{"generic pre-tool plan = plan", "gemini", "PreToolUse", "ExitPlanMode", w(KindPlan)},
		{"generic pre-tool question = question", "gemini", "PreToolUse", "AskUserQuestion", w(KindQuestion)},
		{"generic permission = permission", "gemini", "PermissionRequest", "", w(KindPermission)},
		{"generic stop = stop", "gemini", "Stop", "", Class{Lifecycle: "Stop"}},

		// Kiro: lowercase events + case-insensitive internal tool aliases,
		// scoped to source "kiro".
		{"kiro fs_write = permission", "kiro", "preToolUse", "fs_write", w(KindPermission)},
		{"kiro execute_bash = permission", "kiro", "preToolUse", "execute_bash", w(KindPermission)},
		{"kiro read_file = telemetry", "kiro", "preToolUse", "read_file", tele},
		{"kiro Bash (canonical) = permission", "kiro", "preToolUse", "Bash", w(KindPermission)},
		{"kiro post-tool = resume", "kiro", "postToolUse", "fs_write", resume},

		// kiro's lowercase alias must NOT broaden another agent.
		{"generic lowercase fs_write = telemetry", "gemini", "PreToolUse", "fs_write", tele},

		// Fully unknown.
		{"unknown source+event = telemetry", "nope", "Whatever", "", tele},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classify(c.source, c.ev, c.tool); got != c.want {
				t.Fatalf("classify(%q,%q,%q) = %+v, want %+v", c.source, c.ev, c.tool, got, c.want)
			}
		})
	}
}

func TestStaleStop(t *testing.T) {
	cases := []struct {
		name                     string
		event, active, evSession string
		want                     bool
	}{
		{"superseded stop ignored", "Stop", "sessA", "sessB", true},
		{"matching stop applies", "Stop", "sessA", "sessA", false},
		{"no active session (pre-upgrade)", "Stop", "", "sessB", false},
		{"no event session (non-claude)", "Stop", "sessA", "", false},
		{"non-stop event never stale", "UserPromptSubmit", "sessA", "sessB", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := staleStop(c.event, c.active, c.evSession); got != c.want {
				t.Fatalf("staleStop(%q,%q,%q)=%v want %v", c.event, c.active, c.evSession, got, c.want)
			}
		})
	}
}

// IsAskKind separates "the agent asked" from "gtmux inferred it from the screen". Both
// write the same waiting marker, and conflating them is what let an agent's prose list
// reach the phone as an approval card (2026-08-10).
func TestIsAskKind(t *testing.T) {
	for _, k := range []string{"permission", "plan", "question"} {
		if !IsAskKind(k) {
			t.Errorf("IsAskKind(%q) = false — the agent DID ask; the card must be offered", k)
		}
	}
	// The slow tick's own screen verdicts. These are gtmux talking, not the agent.
	for _, k := range []string{"startup", "draft"} {
		if IsAskKind(k) {
			t.Errorf("IsAskKind(%q) = true — that kind is a screen inference, not an ask", k)
		}
	}
	// No evidence of an ask reads as no ask: no card, and the user replies in the
	// terminal. A card offering choices nobody offered is the worse failure.
	for _, k := range []string{"", "pending", "whatever-a-future-writer-invents"} {
		if IsAskKind(k) {
			t.Errorf("IsAskKind(%q) = true — an unrecognized kind is not evidence of an ask", k)
		}
	}
}

// Every waiting-marker kind the HOOK can write must be an ask kind — otherwise this gate
// would silently drop a real approval card. Guards the two from drifting apart.
func TestEveryHookWaitKindIsAnAskKind(t *testing.T) {
	for _, k := range []Kind{KindPermission, KindPlan, KindQuestion} {
		if !IsAskKind(string(k)) {
			t.Errorf("hook writes kind %q but IsAskKind says it is not an ask — the approval card would vanish", k)
		}
	}
}
