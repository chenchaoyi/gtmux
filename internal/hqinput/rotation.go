package hqinput

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// ReadRotationFile shares the bounded read-only private-file boundary with the
// resident maintenance reader. It never follows or blocks on a special leaf.
func ReadRotationFile() ([]byte, error) {
	f, err := state.OpenPrivateRead(state.WorkflowPath("hqwake", "rotate-pending.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil {
		return nil, err
	}
	if len(b) > 8192 {
		return nil, errors.New("HQ rotation reservation exceeds read budget")
	}
	return b, nil
}

type RotationOpportunity uint8

const (
	RotationUnreserved RotationOpportunity = iota
	RotationReset
	RotationHandoff
)

// RotationWindow reads a bounded input reservation. Callers hold Acquire.
func RotationWindow(pane string, now int64) (RotationOpportunity, error) {
	b, err := ReadRotationFile()
	if errors.Is(err, os.ErrNotExist) {
		return RotationUnreserved, nil
	}
	if err != nil {
		return RotationUnreserved, err
	}
	var r struct {
		Pane, Agent, Retiring, Input               string
		CreatedAt, SentAt, SwitchedAt, BootstrapAt int64
		ReserveBrief                               bool
	}
	if json.Unmarshal(b, &r) != nil || r.Pane == "" || r.Agent == "" || r.Retiring == "" || r.Input == "" || r.CreatedAt <= 0 || r.SentAt < 0 ||
		r.SwitchedAt < 0 || r.SwitchedAt > 0 && (r.SentAt == 0 || r.SwitchedAt < r.SentAt) || r.BootstrapAt < 0 {
		return RotationUnreserved, errors.New("HQ rotation reservation is unreadable")
	}
	if r.Pane != pane {
		return RotationUnreserved, nil
	}
	if r.SentAt == 0 {
		return RotationReset, nil
	}
	if r.Agent != "codex" || !r.ReserveBrief && r.BootstrapAt == 0 {
		return RotationUnreserved, nil
	}
	start, limit := r.SentAt, RotationSettleWait
	if r.SwitchedAt > 0 {
		start, limit = r.SwitchedAt, RotationReadyWait
	}
	if now-start < int64(limit/time.Second) {
		return RotationHandoff, nil
	}
	return RotationUnreserved, nil
}

const (
	RotationSettleWait = 45 * time.Second
	RotationReadyWait  = 3 * time.Minute
)

// RotationBrief binds a queued startup prompt to the sent rotation that
// reserved it and to its one verified Codex successor.
type RotationBrief struct {
	Request, Pane, Agent, Home, Session, Stamp, Instance string
	ClientPID                                            int
	Deadline                                             int64
}

func (b RotationBrief) Valid() bool {
	return b.Request != "" && b.Pane != "" && b.Agent == "codex" && b.Home != "" &&
		b.Session != "" && b.Stamp != "" && b.Instance != "" && b.ClientPID > 0 && b.Deadline > 0
}

// MatchesRotationBrief checks the original reservation under the HQ input
// lease. An absent, unreadable or superseded request cannot authorize a wake.
func MatchesRotationBrief(brief RotationBrief) bool {
	raw, err := ReadRotationFile()
	if err != nil {
		return false
	}
	var r struct {
		Pane, Agent, Retiring, Successor, TargetStamp, BootstrapID, BootstrapHome, BootstrapInstance string
		BootstrapPID                                                                                 int
		SentAt, SwitchedAt, BootstrapAt                                                              int64
		SubmittedAtNS                                                                                int64
	}
	return json.Unmarshal(raw, &r) == nil &&
		r.SubmittedAtNS > 0 && time.Unix(0, r.SubmittedAtNS).Unix() >= r.SentAt &&
		time.Unix(0, r.SubmittedAtNS).Unix() <= r.BootstrapAt &&
		r.BootstrapID == brief.Request && r.BootstrapHome == brief.Home &&
		r.BootstrapInstance == brief.Instance && r.BootstrapPID == brief.ClientPID &&
		r.Pane == brief.Pane && r.Agent == brief.Agent && r.Successor == brief.Session &&
		r.TargetStamp == brief.Stamp && r.Retiring != "" && r.Retiring != r.Successor &&
		r.SentAt > 0 && r.SwitchedAt >= r.SentAt && r.BootstrapAt >= r.SwitchedAt &&
		r.SwitchedAt == brief.Deadline-int64(RotationReadyWait/time.Second) &&
		r.BootstrapAt < brief.Deadline
}
