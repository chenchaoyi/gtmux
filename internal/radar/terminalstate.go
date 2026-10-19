package radar

import (
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
)

type terminalState struct {
	status, reason string
	since          int64
}

var terminalHistory = dsh.Sessions.Resolve
var terminalStuckDispatch = StuckDispatchKind

// terminalStatus consumes a freshly admitted lifecycle-aware attachment. Native
// hook evidence wins; an explicit later durable boundary can repair a missed hook.
// Default history idle, inferred markers, screen activity and CPU are not proof.
func terminalStatus(a agentclient.Attachment, now time.Time) terminalState {
	out := terminalState{status: "running"}
	v, ok := agentclient.ReadLifecycle(a, now)
	if ok {
		out = terminalState{v.Status(), v.EndReason(), v.Since / 1000}
		if out.status == "running" {
			out.since = 0
		}
	}
	if a.Agent == "dsh" && a.HomeID == dsh.HomeID() {
		s, err := terminalHistory(a.SessionID)
		if err == nil && s.ID == a.SessionID && s.HomeID == a.HomeID && s.Origin != "subagent" && hqpane.SameDir(s.Cwd, a.Cwd) &&
			s.TurnAt >= max(a.StartedAt, a.BindingEpoch) && s.TurnAt <= now.UnixMilli()+1000 &&
			(!ok || s.TurnAt > v.ObservedAt) && (s.State == "working" || s.State == "idle") {
			out = terminalState{s.State, s.EndReason, s.TurnAt / 1000}
		}
	}
	return out
}
