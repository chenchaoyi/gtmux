package hook

import (
	"encoding/json"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// classifyPayload keeps an agent's terminal outcome separate from a normal Stop.
// Tool failures and recoverable errors do not end the parent turn.
func classifyPayload(agent, event, tool string, raw []byte) Class {
	c := classify(agent, event, tool)
	var p struct {
		Status           string `json:"status"`
		Recoverable      *bool  `json:"recoverable"`
		NotificationType string `json:"notification_type"`
	}
	_ = json.Unmarshal(raw, &p)
	if agent == "gemini" && event == "Notification" {
		if p.NotificationType == "ToolPermission" {
			return Class{Lifecycle: "Waiting", Kind: KindPermission, Actionable: true}
		}
		return Class{}
	}
	if agent == "cursor" && c.Lifecycle == "Stop" {
		switch p.Status {
		case "aborted":
			return Class{Lifecycle: "Interrupt"}
		case "error":
			return Class{Lifecycle: "StopFailure"}
		case "completed":
			return c
		default:
			return Class{}
		}
	}
	if agent == "copilot" && (event == "ErrorOccurred" || event == "errorOccurred") {
		if p.Recoverable != nil && !*p.Recoverable {
			return Class{Lifecycle: "StopFailure"}
		}
		return Class{}
	}
	return c
}

func hookError(raw json.RawMessage, fallback string) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var object struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &object) == nil && object.Message != "" {
		return object.Message
	}
	return fallback
}

// A cancellation/error must survive the server's independent working-to-idle
// poller. A subsequent actual turn clears it; queued input does not.
func applyTurnOutcome(event, pane string) {
	path := state.TurnOutcomePath(pane)
	switch event {
	case "Interrupt":
		_ = state.WriteMarker(path, "interrupted")
	case "StopFailure":
		_ = state.WriteMarker(path, "error")
	case "Stop", "UserPromptSubmit", "TurnStarted", "SessionStart", "SessionEnd":
		state.Remove(path)
	}
}

func turnOutcome(event string) string {
	switch strings.TrimSpace(event) {
	case "Interrupt":
		return "interrupted"
	case "StopFailure":
		return "error"
	default:
		return ""
	}
}

func staleTurnEvent(event, current, incoming string) bool {
	return event != "UserPromptSubmit" && event != "TurnStarted" && event != "SessionStart" && event != "SessionEnd" &&
		current != "" && incoming != "" && current != incoming
}

func acceptPaneTurn(event, pane, turnID string, sameSessionTurn bool) bool {
	if staleTurnEvent(event, state.ReadMarker(state.TurnIDPath(pane)), turnID) {
		return false
	}
	// A tool callback delivered after cancellation cannot restart the cancelled
	// turn. Only a real prompt/turn-start event may do that.
	if (event == "Resumed" || event == "Waiting" || event == "Stop" || event == "PostCompact") && !state.Exists(state.ActivePath(pane)) && state.Exists(state.TurnOutcomePath(pane)) {
		return false
	}
	switch event {
	case "UserPromptSubmit", "TurnStarted":
		if turnID != "" {
			_ = state.WriteMarker(state.TurnIDPath(pane), turnID)
		} else {
			state.Remove(state.TurnIDPath(pane))
		}
	case "SessionEnd":
		state.Remove(state.TurnIDPath(pane))
	case "SessionStart":
		if !sameSessionTurn {
			state.Remove(state.TurnIDPath(pane))
		}
	}
	return true
}
