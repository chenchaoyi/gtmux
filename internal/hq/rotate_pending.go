package hq

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// A rotation request is durable because the requesting HQ turn must finish before
// Codex will accept /new. The resident serve owns delivery and settlement; the
// command only queues, and never calls a pasted slash command "success".
type rotationRequest struct {
	Pane      string `json:"pane"`
	Agent     string `json:"agent"`
	Retiring  string `json:"retiring"`
	Input     string `json:"input"`
	CreatedAt int64  `json:"createdAt"`
	SentAt    int64  `json:"sentAt,omitempty"`
}

const (
	rotationReadyDelay = 3 * time.Second
	rotationSettleWait = 45 * time.Second
	rotationQueueLimit = 30 * time.Minute
)

func rotationPendingPath() string { return filepath.Join(state.Dir(), "hqwake", "rotate-pending.json") }

func readRotationRequest() (rotationRequest, bool, error) {
	b, err := os.ReadFile(rotationPendingPath())
	if errors.Is(err, os.ErrNotExist) {
		return rotationRequest{}, false, nil
	}
	if err != nil {
		return rotationRequest{}, false, err
	}
	var r rotationRequest
	if err := json.Unmarshal(b, &r); err != nil {
		return rotationRequest{}, false, fmt.Errorf("unreadable pending HQ rotation: %w", err)
	}
	if r.Pane == "" || r.Retiring == "" || r.Agent == "" || r.Input == "" {
		return rotationRequest{}, false, errors.New("pending HQ rotation is missing its identity or reset input")
	}
	return r, true, nil
}

// writeRotationFile uses same-directory rename for updates. The initial request
// uses an atomic hard link so two CLI processes cannot both create it.
func writeRotationFile(r rotationRequest, create bool) error {
	path := rotationPendingPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "rotate-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	b, err := json.Marshal(r)
	if err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if create {
		return os.Link(f.Name(), path)
	}
	return os.Rename(f.Name(), path)
}

// RequestHQRotation queues a single session-bound request. A second request for
// that session is the same pending request; it never types a second /new.
func RequestHQRotation() (rotationRequest, bool, string) {
	pane := hqpane.Find()
	if pane == "" {
		return rotationRequest{}, false, "no live HQ pane"
	}
	agent, sid := hqSessionRef(pane)
	if sid == "" {
		return rotationRequest{}, false, "HQ session ID is not known; cannot verify a successor"
	}
	input := rotateInput(agent)
	if input == "" {
		return rotationRequest{}, false, "this HQ agent has no known reset command"
	}
	return queueHQRotation(rotationRequest{Pane: pane, Agent: agent, Retiring: sid, Input: input, CreatedAt: time.Now().Unix()})
}

func queueHQRotation(r rotationRequest) (rotationRequest, bool, string) {
	if current, exists, err := readRotationRequest(); err != nil {
		return rotationRequest{}, false, err.Error()
	} else if exists {
		if current.Pane == r.Pane && current.Agent == r.Agent && current.Retiring == r.Retiring {
			return current, true, ""
		}
		return rotationRequest{}, false, "another HQ rotation is still pending"
	}
	if err := writeRotationFile(r, true); err != nil {
		if errors.Is(err, os.ErrExist) {
			return queueHQRotation(r) // another CLI won creation; report its request
		}
		return rotationRequest{}, false, err.Error()
	}
	events.AuditRotateRequested(r.Agent, r.Retiring, r.Input, r.CreatedAt)
	return r, true, ""
}

// ProcessPendingHQRotation runs only from serve's slow tick. An unfinished turn,
// a draft, or an unreadable composer leaves the durable request untouched.
func ProcessPendingHQRotation() {
	r, exists, err := readRotationRequest()
	if err != nil {
		return // never type when the request cannot be read; doctor/log can inspect it
	}
	if !exists {
		return
	}
	pane := hqpane.Find()
	if pane == "" {
		failRotation(r, "HQ pane disappeared", time.Now().Unix())
		return
	}
	agent, sid := hqSessionRef(pane)
	advanceHQRotation(r, pane, agent, sid, dispatchbridge.DispatchIO(pane), transcript.CodexLastTurnBoundary, time.Now())
}

func advanceHQRotation(r rotationRequest, pane, agent, sid string, io dispatch.IO,
	boundary func(string) (string, time.Time), now time.Time) {
	if pane != r.Pane || agent != r.Agent {
		failRotation(r, "HQ pane or agent changed", now.Unix())
		return
	}
	if r.SentAt != 0 {
		if sid != "" && sid != r.Retiring {
			events.AuditRotateConfirmed(r.Agent, r.Retiring, sid, r.Input, now.Unix())
			_ = os.Remove(rotationPendingPath())
			return
		}
		if now.Unix()-r.SentAt >= int64(rotationSettleWait.Seconds()) {
			failRotation(r, "reset was sent but no new session ID appeared", now.Unix())
		}
		return
	}
	if sid != r.Retiring {
		failRotation(r, "HQ session changed before reset was sent", now.Unix())
		return
	}
	if now.Unix()-r.CreatedAt >= int64(rotationQueueLimit.Seconds()) {
		failRotation(r, "no safe idle input box within 30 minutes", now.Unix())
		return
	}
	if !rotationTurnFinished(r, boundary, now) || !dispatch.BoxEmpty(io) {
		return
	}
	// Recheck the session's boundary after the two-frame box guard, immediately
	// before typing. A new turn may have started while the screen was sampled.
	if !rotationTurnFinished(r, boundary, now) {
		return
	}
	// Persist the attempt BEFORE either key operation. A crash between Paste and
	// persistence would otherwise leave a queued request that could paste a second
	// reset after Codex discards or submits the first draft. An uncertain attempt
	// may time out as failed, but it is never retried automatically.
	r.SentAt = now.Unix()
	if err := writeRotationFile(r, false); err != nil {
		failRotation(r, "could not record reset attempt; no keys were sent", now.Unix())
		return
	}
	if err := io.Paste(r.Input); err != nil {
		failRotation(r, "could not paste reset command; inspect HQ input box", now.Unix())
		return
	}
	if err := io.Enter(); err != nil {
		failRotation(r, "could not submit reset command; inspect HQ input box", now.Unix())
	}
}

func rotationTurnFinished(r rotationRequest, boundary func(string) (string, time.Time), now time.Time) bool {
	if state.Exists(state.WaitingPath(r.Pane)) || state.Exists(state.ActivePath(r.Pane)) {
		return false
	}
	if r.Agent == "codex" {
		kind, at := boundary(r.Retiring)
		return kind == "task_complete" && !at.IsZero() && !at.After(now.Add(-rotationReadyDelay))
	}
	// Other agents have no Codex-style task boundary. Require their Stop hook's
	// finished marker; a visible composer alone is not turn completion evidence.
	return state.Exists(state.FinishedPath(r.Pane))
}

func failRotation(r rotationRequest, reason string, now int64) {
	events.AuditRotateFailed(r.Agent, r.Retiring, reason, now)
	_ = os.Remove(rotationPendingPath())
}
