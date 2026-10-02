package hook

import (
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func codexSubmittedBinding(prompt, sid string, now int64) string {
	intent, ok := resume.CodexBindingForPrompt(prompt, now)
	if !ok {
		return ""
	}
	live, ok := resume.LiveCodexBindingTarget(intent.Pane)
	if !ok {
		return ""
	}
	if err := resume.CompleteCodexBinding(intent, live, sid, now); err != nil {
		diag.For("hook").Info("codex.binding.deferred", "submitted task binding awaits verified ownership", "pane", intent.Pane, "agent_session", sid, "error", err)
		return ""
	}
	diag.For("hook").Info("codex.binding.confirmed", "submitted task bound to its live Codex pane", "pane", live.Pane, "agent_session", sid)
	return live.Pane
}
