package app

import (
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

var workspaceResolveHQ = workspaceHQTarget
var workspaceDispatch = dispatchbridge.DispatchIO

// Working clients can accept an explicit user instruction through the ordinary
// guarded dispatcher. Permission menus and unknown states still hold delivery.
func workspaceInputHold(status string) string {
	if status == "idle" || status == "working" {
		return ""
	}
	return "hq-waiting"
}

func workspaceConversation(p radar.Pane) (hqworkspace.Conversation, bool) {
	if !strings.HasPrefix(p.PaneID, "%") || p.Loc == "" {
		return hqworkspace.Conversation{}, false
	}
	r, ok := resume.LoadLive(p.Loc)
	if !ok || r.SessionID == "" || r.Agent != agents.KeyForLabel(p.Agent) || r.Agent == "codex" && r.HomeID != "" && r.HomeID != transcript.CodexHomeID() {
		return hqworkspace.Conversation{}, false
	}
	// The live locator/foreground agent and hook ownership must agree.
	if loc := tmux.Display(p.PaneID, "#{session_name}:#{window_index}.#{pane_index}"); loc != p.Loc {
		return hqworkspace.Conversation{}, false
	}
	if sid := state.ReadMarker(state.ActivePath(p.PaneID)); sid != "" && sid != r.SessionID {
		return hqworkspace.Conversation{}, false
	}
	home := r.HomeID
	if r.Agent == "codex" && home == "" {
		home = transcript.CodexHomeID()
	}
	return hqworkspace.Conversation{Agent: r.Agent, HomeID: home, SessionID: r.SessionID, Name: p.Task, Pane: p.PaneID, Loc: p.Loc}, true
}

func workspaceHQTarget() (hqworkspace.Conversation, string) {
	for _, p := range radar.GatherAgents() {
		if p.Role() != "supervisor" {
			continue
		}
		c, ok := workspaceConversation(p)
		if !ok {
			return c, "hq-identity-unavailable"
		}
		if dispatchbridge.ShellCommands[tmux.Display(p.PaneID, "#{pane_current_command}")] && radar.AgentDriverKey(p.PaneID) == "" {
			return c, "hq-input-unavailable"
		}
		if why := workspaceInputHold(p.Status); why != "" {
			return c, why
		}
		return c, ""
	}
	return hqworkspace.Conversation{}, "hq-offline"
}

func workspaceIsHQCaller() bool {
	return currentCallerIsHQ()
}

func pumpWorkspace(s *hqworkspace.Store, now int64) (bool, error) {
	d, err := s.Read()
	if err != nil {
		return false, err
	}
	pending := false
	for _, m := range d.Messages {
		if m.Delivery == "pending" || m.Delivery == "submitting" {
			pending = true
			break
		}
	}
	if !pending {
		return false, nil
	}
	return s.Pump(hqworkspace.DeliveryIO{Resolve: workspaceResolveHQ, SendWithID: workspaceDeliverWithID}, now)
}

func workspaceDeliver(target hqworkspace.Conversation, payload string) hqworkspace.Delivery {
	return workspaceDeliverWithID(target, payload, "")
}

func workspaceDeliverWithID(target hqworkspace.Conversation, payload, deliveryID string) hqworkspace.Delivery {
	release, acquired, err := hqinput.Acquire()
	if err != nil {
		return hqworkspace.Delivery{State: "hq-input-lock-failed", Evidence: err.Error(), SafeHold: true}
	}
	if !acquired {
		return hqworkspace.Delivery{State: "hq-input-locked", SafeHold: true}
	}
	defer release()
	hold := func() string {
		c, why := workspaceResolveHQ()
		if !c.Same(target) || c.Pane != target.Pane {
			return "hq-session-changed"
		}
		return why
	}
	if why := hold(); why != "" {
		return hqworkspace.Delivery{State: why, SafeHold: true}
	}
	cmd := target.Agent
	io := workspaceDispatch(target.Pane, cmd)
	io.Hold = hold
	opts := dispatchbridge.DeliverOpts(target.Pane, cmd, false, dispatch.LoadTuning())
	opts.DeliveryID = deliveryID
	result := dispatch.Deliver(io, opts, payload)
	events.AuditSend(target.Pane, string(result.State), payload, time.Now().Unix())
	safe := result.State == dispatch.StateRefusedDraft || result.State == dispatch.StateRefusedWaiting && !strings.HasPrefix(result.Evidence, dispatch.EvidenceHeldBeforeEnter) && !strings.HasPrefix(result.Evidence, dispatch.EvidenceHeldBeforeRetry)
	return hqworkspace.Delivery{State: string(result.State), Evidence: result.Evidence, SafeHold: safe}
}
