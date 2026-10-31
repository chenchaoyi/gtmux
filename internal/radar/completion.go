package radar

import (
	"fmt"
	"strconv"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// paneCompletion keeps lifecycle proof separate from idleSince's display cache.
// Unknown/legacy markers may still supply an idle time, but never a done alert.
func paneCompletion(pane, loc, cwd, agent string, pid int, status string, attachment agentclient.Attachment, scope *transcript.CodexPathScope) (id string, at int64, instance string) {
	key := agents.KeyForLabel(agent)
	instance = fmt.Sprintf("%s:%s:%d", key, loc, pid)
	if attachment.NativeSession && agentclient.ProvidesLifecycle(attachment.Client) {
		instance += fmt.Sprintf(":%s:%s:%s:%d", attachment.HomeID, attachment.SessionID, attachment.Instance, attachment.BindingEpoch)
		if status == "idle" {
			v := terminalStatus(attachment, time.Now())
			if v.status == "idle" && v.reason == "" {
				return v.completionID, v.completedAt, instance
			}
		}
		return "", 0, instance
	}
	rec, bound := boundResume(loc, agent)
	if bound {
		instance += ":" + rec.HomeID + ":" + rec.SessionID
	}
	if status != "idle" || state.Exists(state.ActivePath(pane)) || state.Exists(state.WaitingPath(pane)) {
		return "", 0, instance
	}
	if c, ok := state.ReadCompletion(pane); ok && c.Agent == key &&
		(!bound || c.SessionID == "" || c.SessionID == rec.SessionID) &&
		(c.TurnID == "" || state.ReadMarker(state.TurnIDPath(pane)) == c.TurnID) {
		id, at = "hook:"+strconv.FormatInt(c.At, 10), c.At
	}
	if key == "codex" && bound && resume.CodexCwdMatches(pane, rec, cwd) {
		kind, boundary := scope.CodexLastTurnBoundary(rec.SessionID)
		if boundary.UnixNano() > at && !boundary.After(time.Now().Add(time.Second)) {
			if kind == "task_complete" && scopedLastMessageTime(scope, "codex", rec.SessionID) <= boundary.Unix() {
				id, at = "codex:"+rec.SessionID+":"+strconv.FormatInt(boundary.UnixNano(), 10), boundary.UnixNano()
			} else if kind == "task_started" || kind == "turn_aborted" {
				id, at = "", 0
			}
		}
	}
	return id, at, instance
}

func nativeCompletion(r native.Record, status string, scope *transcript.CodexPathScope) (id string, at int64) {
	if status != "idle" || r.EndReason != "" {
		return "", 0
	}
	if r.Event == "Stop" {
		id, at = "native:"+r.Revision+":"+r.TurnID+":"+strconv.FormatInt(r.UpdatedAt, 10), r.UpdatedAt*int64(time.Second)
	}
	if r.Agent == "codex" && nativeReadableScope(r) {
		reader := transcript.CodexLastTurnBoundary
		if scope != nil {
			reader = scope.CodexLastTurnBoundary
		}
		kind, boundary := reader(r.SessionID)
		if kind == "task_complete" && boundary.Unix() >= r.UpdatedAt && !boundary.After(time.Now().Add(time.Second)) {
			return "codex:" + r.SessionID + ":" + strconv.FormatInt(boundary.UnixNano(), 10), boundary.UnixNano()
		}
	}
	if r.Agent == "dsh" && nativeReadableScope(r) {
		if s, err := dsh.Sessions.Resolve(r.SessionID); err == nil && s.ID == r.SessionID && s.HomeID == r.HomeID &&
			s.Origin != "subagent" && hqpane.SameDir(s.Cwd, r.Cwd) && s.State == "idle" && s.EndReason == "" &&
			s.TurnAt > 0 && s.TurnAt/1000 >= r.UpdatedAt && s.TurnAt <= time.Now().UnixMilli()+1000 {
			return "dsh:" + r.SessionID + ":" + strconv.FormatInt(s.TurnAt, 10), s.TurnAt * int64(time.Millisecond)
		}
	}
	return id, at
}
