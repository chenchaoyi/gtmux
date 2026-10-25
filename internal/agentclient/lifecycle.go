package agentclient

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Lifecycle is native event evidence, separate from the radar's inferred marker
// caches. Its attachment must still match a freshly verified live attachment.
// It neither proves process liveness nor grants input authority.
type Lifecycle struct {
	Version    int        `json:"version"`
	Attachment Attachment `json:"attachment"`
	Event      string     `json:"event"`
	TurnID     string     `json:"turn_id,omitempty"`
	ObservedAt int64      `json:"observed_at"` // Unix milliseconds
	Since      int64      `json:"since"`
}

func (v Lifecycle) Status() string {
	switch v.Event {
	case "UserPromptSubmit", "TurnStarted", "Resumed":
		return "working"
	case "Waiting":
		return "waiting"
	case "Stop", "Interrupt", "StopFailure":
		return "idle"
	case "SessionStart", "SessionEnd":
		return "running" // known attachment, unknown turn state
	default:
		return ""
	}
}

func (v Lifecycle) EndReason() string {
	switch v.Event {
	case "Interrupt":
		return "interrupted"
	case "StopFailure":
		return "error"
	default:
		return ""
	}
}

func lifecycleAttachment(a Attachment) bool {
	return a.Version == ProtocolVersion && ProvidesLifecycle(a.Client) && a.NativeSession && panePattern.MatchString(a.Pane) &&
		a.Agent != "" && a.HomeID != "" && a.SessionID != "" && a.PID > 0 && instancePattern.MatchString(a.Instance) &&
		a.ServerEpoch != "" && a.StartedAt > 0 && a.BindingEpoch > 0
}

func (v Lifecycle) valid(now time.Time) bool {
	return v.Version == ProtocolVersion && lifecycleAttachment(v.Attachment) && v.Status() != "" &&
		len(v.TurnID) <= 256 && !strings.ContainsAny(v.TurnID, "\x00\r\n") &&
		v.Since >= max(v.Attachment.StartedAt, v.Attachment.BindingEpoch) &&
		v.Since <= v.ObservedAt && v.ObservedAt <= now.UnixMilli()+1000
}

// ReadLifecycle requires the current attachment supplied by Load/All. A stale
// record cannot migrate across a client incarnation, foreground, home or pane.
func ReadLifecycle(a Attachment, now time.Time) (Lifecycle, bool) {
	var v Lifecycle
	if !lifecycleAttachment(a) || ReadOwnedJSON(state.TerminalLifecyclePath(a.Pane), &v, 16384) != nil ||
		!v.valid(now) || !Same(a, v.Attachment) || (a.StartedAt != v.Attachment.StartedAt || a.Cwd != v.Attachment.Cwd) {
		return Lifecycle{}, false
	}
	return v, true
}

// ObserveLifecycle accepts canonical events ONLY after the hook has verified the
// native sender and accepted its turn. Native bridge callbacks are serialized.
// The bounded lease serializes the shared snapshot with other hook processes;
// failure drops this evidence with a caller diagnostic, never blocking a model.
func ObserveLifecycle(a Attachment, event, turnID string, now time.Time) error {
	v := Lifecycle{ProtocolVersion, a, event, turnID, now.UnixMilli(), now.UnixMilli()}
	if v.Status() == "" {
		return nil
	}
	if !v.valid(now) {
		return errors.New("invalid native lifecycle evidence")
	}
	var lease *state.Lease
	var err error
	for i := 0; i < 20; i++ {
		lease, err = state.TryLease("terminal-lifecycle")
		if !errors.Is(err, state.ErrLeaseHeld) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	defer lease.Close()
	var previous Lifecycle
	if ReadOwnedJSON(state.TerminalLifecyclePath(a.Pane), &previous, 16384) == nil && previous.valid(now) {
		// An already accepted newer incarnation/foreground wins a late callback.
		if previous.Attachment.StartedAt > a.StartedAt || previous.Attachment.BindingEpoch > a.BindingEpoch {
			return nil
		}
		if Same(a, previous.Attachment) && a.StartedAt == previous.Attachment.StartedAt && a.Cwd == previous.Attachment.Cwd {
			if previous.ObservedAt > v.ObservedAt || event == "SessionStart" {
				return nil
			}
			start := event == "UserPromptSubmit" || event == "TurnStarted"
			if !start && event != "SessionEnd" && previous.TurnID != "" && turnID != "" && previous.TurnID != turnID {
				return nil
			}
			// A delayed tool callback/duplicate end cannot revive or re-date a
			// settled turn. A distinct real prompt/start opens the next turn.
			if previous.Status() == "idle" && event != "SessionEnd" && (!start || turnID != "" && turnID == previous.TurnID) {
				return nil
			}
			if v.Status() == previous.Status() && !(start && turnID != "" && previous.TurnID != "" && turnID != previous.TurnID) {
				v.Since = previous.Since
			}
			if v.TurnID == "" && event != "SessionEnd" {
				v.TurnID = previous.TurnID
			}
		}
	}
	if err := state.MkdirAll(state.TerminalLifecycleDir(), 0700); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > 16384 {
		return errors.New("native lifecycle evidence exceeds limit")
	}
	return atomicPrivate(state.TerminalLifecyclePath(a.Pane), b)
}
