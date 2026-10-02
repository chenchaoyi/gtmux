package app

import (
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

// A fresh interactive Codex conversation needs evidence independent of the
// shared app-server's inherited pane. Resumed conversations already have an ID.
func spawnBindingPayload(pane, agent, payload string) string {
	key, _ := resume.FromCommand(agent)
	if key != "codex" {
		return payload
	}
	live, ok := resume.LiveCodexBindingTarget(pane)
	if !ok {
		diag.For("cli").Warn("codex.binding.unavailable", "spawn could not identify its live Codex target", "pane", pane)
		return payload
	}
	if rec, ok := resume.Load(live.Loc); ok && rec.Agent == "codex" && rec.SessionID != "" {
		return payload
	}
	wire, err := resume.PrepareCodexBinding(live, payload, time.Now().Unix())
	if err != nil {
		diag.For("cli").Warn("codex.binding.unavailable", "spawn could not persist its Codex binding intent", "pane", pane, "error", err)
		return payload
	}
	return wire
}
