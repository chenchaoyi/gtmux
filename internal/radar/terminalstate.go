package radar

import (
	"strconv"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
)

type terminalState struct {
	status, reason string
	since          int64
	completionID   string
	completedAt    int64
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
		out = terminalState{status: v.Status(), reason: v.EndReason(), since: v.Since / 1000}
		if v.Event == "Stop" {
			out.completionID = "terminal:" + v.TurnID + ":" + strconv.FormatInt(v.ObservedAt, 10)
			out.completedAt = v.ObservedAt * int64(time.Millisecond)
		}
		if out.status == "running" {
			out.since = 0
		}
	}
	if a.Agent == "dsh" && a.HomeID == dsh.HomeID() {
		s, err := terminalHistory(a.SessionID)
		if err == nil && s.ID == a.SessionID && s.HomeID == a.HomeID && s.Origin != "subagent" && hqpane.SameDir(s.Cwd, a.Cwd) &&
			s.TurnAt >= max(a.StartedAt, a.BindingEpoch) && s.TurnAt <= now.UnixMilli()+1000 &&
			(!ok || s.TurnAt > v.ObservedAt) && (s.State == "working" || s.State == "idle") {
			out = terminalState{status: s.State, reason: s.EndReason, since: s.TurnAt / 1000}
			if s.State == "idle" && s.EndReason == "" {
				out.completionID = "dsh:" + s.ID + ":" + strconv.FormatInt(s.TurnAt, 10)
				out.completedAt = s.TurnAt * int64(time.Millisecond)
			}
		}
	}
	return out
}
