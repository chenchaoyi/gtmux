package hook

import "strings"

// Deterministic agent-event classifier — a Go port of cmux's FeedEventClassifier
// (CLI/FeedEventClassifier.swift), adapted to gtmux's vocabulary. It replaces the
// old "a Notification mid-turn means waiting" timing heuristic with a typed
// (source, event, tool) → semantic decision, so a tool *starting* can never be
// mistaken for an approval request, and the KIND of wait (permission / plan /
// question) is known precisely. Pure + table-driven so adding an agent is data,
// and so the decision is unit-testable without running a hook.

// Kind is what an actionable ("needs you") event is asking for. Empty for
// non-actionable events.
type Kind string

const (
	KindPermission Kind = "permission" // a side-effecting tool needs approve/deny
	KindPlan       Kind = "plan"       // a plan is ready for your approval (ExitPlanMode)
	KindQuestion   Kind = "question"   // the agent is asking you something (AskUserQuestion)
)

// Class is the classification of one raw hook event.
type Class struct {
	// Lifecycle is gtmux's canonical lifecycle event the state machine acts on:
	// "UserPromptSubmit" (turn start), "Stop" (turn end), "Waiting" (blocked on
	// you), or "" (telemetry-only: no state change, no notify).
	Lifecycle string
	// Kind is set only when Lifecycle == "Waiting".
	Kind Kind
	// Actionable mirrors Lifecycle == "Waiting": the user must act.
	Actionable bool
}

// semantic is the user-attention meaning of an event, independent of the
// agent-specific raw name (mirrors cmux's FeedEventSemantic).
type semantic int

const (
	semUnknown                semantic = iota
	semApprovalRequest                 // a real approval is pending → actionable
	semToolStart                       // a tool is starting; agent has a dedicated approval event → telemetry
	semToolStartMaybeApproval          // a tool is starting; no dedicated approval event → escalate side-effecting tools
	semToolEnd
	semPreCompact
	semPostCompact
	semPromptSubmit
	semSubagentStart
	semResponse // the agent finished its turn
	semSubagentResponse
	semSessionStart
	semSessionEnd
	semStatusNotification
)

// classify resolves (source, event, tool) into a gtmux Class. source is the
// agent key ("claude", "codex", …); event is the agent's raw hook event name;
// tool is the tool the event refers to (only the two tool-dependent semantics
// use it), "" when none.
func classify(source, event, tool string) Class {
	sem := eventSemantic(source, event)
	switch sem {
	case semApprovalRequest:
		if k := dedicatedApprovalKind(tool); k != "" {
			return Class{Lifecycle: "Waiting", Kind: k, Actionable: true}
		}
		return Class{Lifecycle: "Waiting", Kind: KindPermission, Actionable: true}
	case semToolStartMaybeApproval:
		if k := dedicatedApprovalKind(tool); k != "" {
			return Class{Lifecycle: "Waiting", Kind: k, Actionable: true}
		}
		if isSideEffectingTool(tool, source) {
			return Class{Lifecycle: "Waiting", Kind: KindPermission, Actionable: true}
		}
		return Class{} // read-only tool start → telemetry
	case semPromptSubmit:
		return Class{Lifecycle: "UserPromptSubmit"}
	case semResponse:
		return Class{Lifecycle: "Stop"}
	default:
		// toolStart, toolEnd, (pre|post)Compact, subagent*, session*,
		// statusNotification, unknown → telemetry; no state change, no notify.
		return Class{}
	}
}

// dedicatedApprovalKind routes a tool that carries its own approval meaning to
// its kind (Claude's plan / question approvals), else "".
func dedicatedApprovalKind(tool string) Kind {
	switch tool {
	case "ExitPlanMode":
		return KindPlan
	case "AskUserQuestion":
		return KindQuestion
	default:
		return ""
	}
}

func eventSemantic(source, event string) semantic {
	if table, ok := agentEventSemantics[source]; ok {
		if s, ok := table[event]; ok {
			return s
		}
		return semUnknown
	}
	if s, ok := genericEventSemantics[event]; ok {
		return s
	}
	return semUnknown
}

// agentEventSemantics is the per-agent (event → semantic) source of truth. The
// key distinction (cmux #4985): agents with a DEDICATED approval event classify
// their pre-tool event as semToolStart (always telemetry); agents whose only
// signal is the pre-tool event use the generic table's semToolStartMaybeApproval
// so side-effecting tools still escalate. Ported from cmux's
// feedEventSemanticRegistry.
var agentEventSemantics = map[string]map[string]semantic{
	"claude": {
		"PermissionRequest": semApprovalRequest,
		"PreToolUse":        semToolStart,
		"PostToolUse":       semToolEnd,
		"PreCompact":        semPreCompact,
		"PostCompact":       semPostCompact,
		"UserPromptSubmit":  semPromptSubmit,
		"SessionStart":      semSessionStart,
		"SessionEnd":        semSessionEnd,
		"Stop":              semResponse,
		"SubagentStart":     semSubagentStart,
		"SubagentStop":      semSubagentResponse,
		"Notification":      semStatusNotification,
	},
	"codex": {
		// Codex runs its own approval reviewer; its PermissionRequest is
		// telemetry so "approve for me" can use Codex's path, not gtmux.
		"PermissionRequest":    semToolStart,
		"permission_request":   semToolStart,
		"PreToolUse":           semToolStart,
		"pre_tool_use":         semToolStart,
		"beforeShellExecution": semToolStart,
		"PostToolUse":          semToolEnd,
		"post_tool_use":        semToolEnd,
		"UserPromptSubmit":     semPromptSubmit,
		"user_prompt_submit":   semPromptSubmit,
		"SessionStart":         semSessionStart,
		"session_start":        semSessionStart,
		"SessionEnd":           semSessionEnd,
		"session_end":          semSessionEnd,
		"Stop":                 semResponse,
		"stop":                 semResponse,
		"agent-turn-complete":  semResponse, // Codex notify payload type
		"SubagentStop":         semSubagentResponse,
		"subagent_stop":        semSubagentResponse,
		"Notification":         semStatusNotification,
		"notification":         semStatusNotification,
	},
	"hermes-agent": {
		// pre_tool_call is a tool *starting* — Hermes raises a separate
		// pre_approval_request for real approvals, so this stays telemetry.
		"pre_tool_call":          semToolStart,
		"post_tool_call":         semToolEnd,
		"pre_approval_request":   semApprovalRequest,
		"post_approval_response": semStatusNotification,
		"pre_llm_call":           semPromptSubmit,
		"post_llm_call":          semResponse,
		"on_session_start":       semSessionStart,
		"on_session_reset":       semSessionStart,
		"on_session_end":         semSessionEnd,
		"on_session_finalize":    semSessionEnd,
	},
	// Kiro: lowercase events, no dedicated approval event → escalate
	// side-effecting tools. Must be registered explicitly or its lowercase
	// names fall to unknown and every approval is dropped.
	"kiro": {
		"preToolUse":       semToolStartMaybeApproval,
		"postToolUse":      semToolEnd,
		"userPromptSubmit": semPromptSubmit,
		"agentSpawn":       semSessionStart,
		"stop":             semResponse,
	},
}

// genericEventSemantics is the fallback for agents without a dedicated entry
// (gemini, copilot, cursor, opencode, …): they expose only a pre-tool event, so
// it carries semToolStartMaybeApproval. Ported from cmux's
// genericFeedEventSemantics.
var genericEventSemantics = map[string]semantic{
	"PreToolUse":           semToolStartMaybeApproval,
	"beforeShellExecution": semToolStartMaybeApproval,
	"PermissionRequest":    semApprovalRequest,
	"PostToolUse":          semToolEnd,
	"PreCompact":           semPreCompact,
	"PostCompact":          semPostCompact,
	"UserPromptSubmit":     semPromptSubmit,
	"SessionStart":         semSessionStart,
	"SessionEnd":           semSessionEnd,
	"Stop":                 semResponse,
	"SubagentStart":        semSubagentStart,
	"SubagentStop":         semSubagentResponse,
	"Notification":         semStatusNotification,
}

// sideEffectingTools mutate state and deserve an approve/deny prompt. Read-only
// tools (Read, Grep, Glob, Task, WebFetch, WebSearch, LS, TodoWrite, …) are
// intentionally excluded so they never flag "needs you". Ported from cmux.
var sideEffectingTools = map[string]bool{
	"Bash": true, "Write": true, "Edit": true, "MultiEdit": true, "NotebookEdit": true,
	"apply_patch": true, "shell": true, "terminal": true, "run_command": true,
	"write_to_file": true, "replace_file_content": true, "multi_replace_file_content": true,
	"manage_task": true, "schedule": true, "ask_permission": true,
	"invoke_subagent": true, "define_subagent": true, "manage_subagents": true,
	"generate_image": true,
}

// kiroSideEffectingAliases are Kiro's lowercase/internal tool names, matched
// case-insensitively but ONLY for source "kiro" so another agent's lowercase
// tool name is never broadened into an approval.
var kiroSideEffectingAliases = map[string]bool{
	"bash": true, "write": true, "edit": true, "multiedit": true, "notebookedit": true,
	"apply_patch": true, "shell": true, "execute_bash": true, "fs_write": true,
	"use_aws": true, "aws": true, "terminal": true, "run_command": true,
	"write_to_file": true, "replace_file_content": true, "multi_replace_file_content": true,
	"manage_task": true, "schedule": true, "ask_permission": true,
	"invoke_subagent": true, "define_subagent": true, "manage_subagents": true,
	"generate_image": true,
}

func isSideEffectingTool(tool, source string) bool {
	if tool == "" {
		return false
	}
	if sideEffectingTools[tool] {
		return true
	}
	if source == "kiro" {
		return kiroSideEffectingAliases[strings.ToLower(tool)]
	}
	return false
}
