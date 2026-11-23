package hqinput

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// ReadRotationFile shares the bounded read-only private-file boundary with the
// resident maintenance reader. It never follows or blocks on a special leaf.
func ReadRotationFile() ([]byte, error) {
	f, err := state.OpenPrivateRead(filepath.Join(state.Dir(), "hqwake", "rotate-pending.json"))
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

// RotationPending reads an input reservation, never authority to reset a session.
// Callers hold Acquire to serialize this observation with request registration.
func RotationPending(pane string) (bool, error) {
	b, err := ReadRotationFile()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var r struct {
		Pane, Agent, Retiring, Input string
		CreatedAt, SentAt            int64
	}
	if json.Unmarshal(b, &r) != nil || r.Pane == "" || r.Agent == "" || r.Retiring == "" || r.Input == "" || r.CreatedAt <= 0 || r.SentAt < 0 {
		return false, errors.New("HQ rotation reservation is unreadable")
	}
	return r.Pane == pane && r.SentAt == 0, nil
}

// RotationReadyWait is the existing confirmation window. The briefing may
// use it as an input deadline without changing how rotation settles.
const RotationReadyWait = 3 * time.Minute

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
	}
	return json.Unmarshal(raw, &r) == nil &&
		r.BootstrapID == brief.Request && r.BootstrapHome == brief.Home &&
		r.BootstrapInstance == brief.Instance && r.BootstrapPID == brief.ClientPID &&
		r.Pane == brief.Pane && r.Agent == brief.Agent && r.Successor == brief.Session &&
		r.TargetStamp == brief.Stamp && r.Retiring != "" && r.Retiring != r.Successor &&
		r.SentAt > 0 && r.SwitchedAt >= r.SentAt && r.BootstrapAt >= r.SwitchedAt &&
		r.SwitchedAt == brief.Deadline-int64(RotationReadyWait/time.Second) &&
		r.BootstrapAt < brief.Deadline
}
