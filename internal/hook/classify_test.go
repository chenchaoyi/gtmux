package hook

import "testing"

func TestClassify(t *testing.T) {
	w := func(k Kind) Class { return Class{Lifecycle: "Waiting", Kind: k, Actionable: true} }
	tele := Class{}

	cases := []struct {
		name             string
		source, ev, tool string
		want             Class
	}{
		// Dedicated-approval agents: a tool *starting* is NEVER an approval
		// (the cmux #4985 fix) — even a side-effecting one.
		{"claude pre-tool bash = telemetry", "claude", "PreToolUse", "Bash", tele},
		{"claude pre-tool exitplan = telemetry", "claude", "PreToolUse", "ExitPlanMode", tele},
		{"claude permission bash = permission", "claude", "PermissionRequest", "Bash", w(KindPermission)},
		{"claude permission plan = plan", "claude", "PermissionRequest", "ExitPlanMode", w(KindPlan)},
		{"claude permission question = question", "claude", "PermissionRequest", "AskUserQuestion", w(KindQuestion)},
		{"claude prompt = turn start", "claude", "UserPromptSubmit", "", Class{Lifecycle: "UserPromptSubmit"}},
		{"claude stop = turn end", "claude", "Stop", "", Class{Lifecycle: "Stop"}},
		{"claude notification = telemetry", "claude", "Notification", "", tele},
		{"claude unknown event = telemetry", "claude", "Frobnicate", "", tele},

		// Codex runs its own approval reviewer → pre-tool/permission are telemetry.
		{"codex pre-tool bash = telemetry", "codex", "PreToolUse", "Bash", tele},
		{"codex permission = telemetry", "codex", "PermissionRequest", "shell", tele},
		{"codex turn complete = stop", "codex", "agent-turn-complete", "", Class{Lifecycle: "Stop"}},

		// Hermes: pre_tool_call telemetry, separate approval event.
		{"hermes pre-tool = telemetry", "hermes-agent", "pre_tool_call", "terminal", tele},
		{"hermes approval = permission", "hermes-agent", "pre_approval_request", "terminal", w(KindPermission)},

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
