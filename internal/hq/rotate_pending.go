package hq

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// A rotation request is durable because the requesting HQ turn must finish before
// Codex will accept /new. The resident serve owns delivery and settlement; the
// command only queues, and never calls a pasted slash command "success".
type rotationRequest struct {
	IdentityUnavailableAt int64  `json:"identityUnavailableAt,omitempty"`
	IdentityCause         string `json:"identityCause,omitempty"`
	IdentityWarnedAt      int64  `json:"identityWarnedAt,omitempty"`
	TargetStamp           string `json:"targetStamp,omitempty"`
	Successor             string `json:"successor,omitempty"`
	SwitchedAt            int64  `json:"switchedAt,omitempty"`
	BootstrapAt           int64  `json:"bootstrapAt,omitempty"`
	Pane                  string `json:"pane"`
	Agent                 string `json:"agent"`
	Retiring              string `json:"retiring"`
	Input                 string `json:"input"`
	CreatedAt             int64  `json:"createdAt"`
	SentAt                int64  `json:"sentAt,omitempty"`
}

const (
	rotationReadyDelay   = 3 * time.Second
	rotationSettleWait   = 45 * time.Second
	rotationQueueLimit   = 30 * time.Minute
	rotationReadyWait    = 3 * time.Minute
	rotationIdentityWait = time.Minute
)

// Use the existing guarded HQ startup channel, after releasing the shared input
// lock. This is the rotation's handoff, not an invented user task or a second /new.
var rotationBootstrap = func(pane, sid, stamp string) {
	_, current := hqSessionRef(pane)
	if current == sid && rotationTargetMatches(stamp, rotationTargetStamp(pane)) {
		hqnudge.Deliver(pane, hqBriefingPrompt())
	}
}

func rotationTargetStamp(pane string) string {
	return strings.TrimSpace(tmux.Display(pane, "#{pid}:#{session_created}:#{pane_pid}"))
}

func rotationTargetMatches(expected, actual string) bool {
	if expected == "" || actual != expected {
		return false
	}
	parts := strings.Split(expected, ":")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if n, err := strconv.ParseInt(part, 10, 64); err != nil || n <= 0 {
			return false
		}
	}
	return true
}

func rotationPendingPath() string { return filepath.Join(state.Dir(), "hqwake", "rotate-pending.json") }

func readRotationRequest() (rotationRequest, bool, error) {
	b, err := hqinput.ReadRotationFile()
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
	if err := state.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := state.CreateTemp(filepath.Dir(path), "rotate-*.tmp")
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
	stamp := rotationTargetStamp(pane)
	if !rotationTargetMatches(stamp, stamp) {
		return rotationRequest{}, false, "HQ process identity could not be verified"
	}
	return queueHQRotation(rotationRequest{Pane: pane, Agent: agent, Retiring: sid, Input: input, CreatedAt: time.Now().Unix(), TargetStamp: stamp})
}

func queueHQRotation(r rotationRequest) (rotationRequest, bool, string) {
	release, acquired, err := hqinput.Acquire()
	if err != nil {
		return rotationRequest{}, false, err.Error()
	}
	if !acquired {
		return rotationRequest{}, false, "HQ input is busy; rotation was not registered"
	}
	defer release()
	return queueHQRotationLocked(r)
}

func queueHQRotationLocked(r rotationRequest) (rotationRequest, bool, string) {
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
			current, exists, readErr := readRotationRequest()
			if readErr != nil {
				return rotationRequest{}, false, readErr.Error()
			}
			if exists && current.Pane == r.Pane && current.Agent == r.Agent && current.Retiring == r.Retiring {
				return current, true, ""
			}
			return rotationRequest{}, false, "HQ rotation changed during registration"
		}
		return rotationRequest{}, false, err.Error()
	}
	events.AuditRotateRequested(r.Agent, r.Retiring, r.Input, r.CreatedAt)
	return r, true, ""
}

// ProcessPendingHQRotation runs before draining on fast and slow ticks. An unfinished turn,
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
		failQueuedRotation(r, "HQ pane disappeared", time.Now().Unix())
		return
	}
	if !rotationTargetMatches(r.TargetStamp, rotationTargetStamp(pane)) {
		failQueuedRotation(r, "HQ target changed or its pre-reboot identity is unavailable; no input sent", time.Now().Unix())
		return
	}
	agent, sid, reason := hqSessionEvidence(pane)
	advanceHQRotation(r, pane, agent, sid, dispatchbridge.DispatchIO(pane), transcript.CodexLastTurnBoundary, time.Now(), reason)
}

func advanceHQRotation(r rotationRequest, pane, agent, sid string, io dispatch.IO,
	boundary func(string) (string, time.Time), now time.Time, identityReason ...string) {
	release, acquired, err := hqinput.Acquire()
	if err != nil {
		diag.ReportStoreFailure("hq-input", err)
		return
	}
	if !acquired {
		return // keep the durable request while another HQ input channel is active
	}
	bootstrap := ""
	defer func() {
		release()
		if bootstrap != "" {
			rotationBootstrap(pane, bootstrap, r.TargetStamp)
		}
	}()
	// A prior tick may have settled or sent this request while we waited for input.
	// Never let an obsolete unsent snapshot repeat a reset or touch its successor.
	current, exists, err := readRotationRequest()
	if err != nil {
		diag.ReportStoreFailure("hq-rotation", err)
		return
	}
	if !exists || current.Pane != r.Pane || current.Agent != r.Agent || current.Retiring != r.Retiring || current.CreatedAt != r.CreatedAt || current.Input != r.Input {
		return
	}
	r = current
	if r.TargetStamp != "" && !rotationTargetMatches(r.TargetStamp, rotationTargetStamp(pane)) {
		failRotation(r, "HQ target changed while waiting for input; no input sent", now.Unix())
		return
	}
	if pane != r.Pane || agent != "" && agent != r.Agent {
		failRotation(r, "HQ pane or agent changed", now.Unix())
		return
	}
	if r.SentAt != 0 {
		if agent == "" {
			failRotation(r, "HQ agent identity unavailable after reset; no reset will be repeated", now.Unix())
			return
		}
		if sid == "" || sid == r.Retiring {
			// Legacy hooks can precede the location record. Modern bridges also bind
			// empty threads, so a changed ID alone is no durability acknowledgement.
			sid = rotationSessionFromEvents(r, rotationEvents(now.Unix()-r.SentAt+5, now.Unix()))
		}
		if sid != "" && sid != r.Retiring {
			if r.Successor != "" && sid != r.Successor {
				failRotation(r, "HQ changed again before its successor became recoverable", now.Unix())
				return
			}
			if r.Successor == "" {
				r.Successor, r.SwitchedAt = sid, now.Unix()
				if err := writeRotationFile(r, false); err != nil {
					diag.ReportStoreFailure("hq-rotation", err)
					return
				}
				diag.For("hq").Info("hq.rotation.switched", "HQ switched; waiting for durable handoff", "session", sid)
			}
			if r.Agent == "codex" {
				kind, at := boundary(sid)
				ready := transcript.CodexRecovery(sid).State == transcript.RecoveryReady && kind != "" && !at.Before(time.Unix(r.SentAt, 0)) && !at.After(now)
				if !ready {
					if now.Unix()-r.SwitchedAt >= int64(rotationReadyWait.Seconds()) {
						failRotation(r, "HQ switched but its handoff/history was not confirmed; no reset will be retried", now.Unix())
						return
					}
					if r.BootstrapAt == 0 && kind == "" && hqBriefingEnabled() &&
						!state.Exists(state.ActivePath(pane)) && !state.Exists(state.WaitingPath(pane)) && dispatch.BoxEmpty(io) {
						r.BootstrapAt = now.Unix()
						if err := writeRotationFile(r, false); err != nil {
							diag.ReportStoreFailure("hq-rotation", err)
							return
						}
						bootstrap = sid
					}
					return
				}
			}
			events.AuditRotateConfirmed(r.Agent, r.Retiring, sid, r.Input, now.Unix())
			_ = os.Remove(rotationPendingPath())
			return
		}
		if now.Unix()-r.SentAt >= int64(rotationSettleWait.Seconds()) {
			failRotation(r, "reset was sent but no new session ID appeared", now.Unix())
		}
		return
	}
	if sid != "" && sid != r.Retiring {
		failRotation(r, "HQ session changed before reset was sent", now.Unix())
		return
	}
	reason := ""
	if len(identityReason) > 0 {
		reason = rotationIdentityCode(identityReason[0])
	}
	if reason == "" && agent == "" {
		reason = "agent-evidence-unavailable"
	}
	if reason == "" && sid == "" {
		reason = "session-evidence-unavailable"
	}
	if active := state.ReadMarker(state.ActivePath(pane)); reason == "" && state.Exists(state.ActivePath(pane)) {
		if active == "" {
			reason = "active-evidence-unavailable"
		} else if active != r.Retiring {
			reason = "active-identity-mismatch"
		}
	}
	if reason != "" {
		holdRotationIdentity(r, reason, now)
		return
	}
	if r.IdentityUnavailableAt != 0 || r.IdentityCause != "" {
		r.IdentityUnavailableAt, r.IdentityCause = 0, ""
		if err := writeRotationFile(r, false); err != nil {
			diag.ReportStoreFailure("hq-rotation", err)
			return
		}
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

// rotationEvents reads the event stream for the confirmation; swappable in tests.
var rotationEvents = events.Read

// rotationSessionFromEvents returns the session a sent reset produced, from the event
// stream: a SessionStart or UserPromptSubmit on the rotation's pane, from the same agent,
// at or after the reset was sent, carrying a session id that is not the retiring one. ""
// when there is none yet. Nothing older than the send, on another pane, from another
// agent, or without a session id counts.
func rotationSessionFromEvents(r rotationRequest, recs []events.Record) string {
	if r.SentAt == 0 {
		return ""
	}
	for i := len(recs) - 1; i >= 0; i-- {
		e := recs[i]
		if e.Ts < r.SentAt || e.Pane != r.Pane || (e.Event != "SessionStart" && e.Event != "UserPromptSubmit") {
			continue
		}
		if e.Agent != r.Agent && agents.KeyForLabel(e.Agent) != r.Agent {
			continue
		}
		if e.AgentSession == "" || e.AgentSession == r.Retiring {
			continue
		}
		return e.AgentSession
	}
	return ""
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

// The early identity refusals keep their original checks. Their failure cleanup
// also owns admission, so it cannot delete a concurrently registered request.
func failQueuedRotation(r rotationRequest, reason string, now int64) {
	release, acquired, err := hqinput.Acquire()
	if err != nil || !acquired {
		return
	}
	defer release()
	failRotation(r, reason, now)
}

func failRotation(r rotationRequest, reason string, now int64) {
	// Every production caller owns the registration/input lease. A stale snapshot
	// cannot repeat an alert or remove a successor request.
	current, exists, err := readRotationRequest()
	if err != nil || !exists || current != r {
		return
	}
	events.AuditRotateFailed(r.Agent, r.Retiring, reason, now)
	notify.Send(notify.Options{
		Kind: "attention", Title: i18n.Tr("HQ handoff did not complete", "HQ 轮换未完成"),
		Subtitle: "gtmux", Message: reason, Pane: r.Pane, Session: "gtmux HQ",
	})
	_ = os.Remove(rotationPendingPath())
}

// Only known, value-free observation codes enter persisted diagnostics or logs.
func rotationIdentityCode(reason string) string {
	switch reason {
	case "", "pane-location-unavailable", "agent-driver-unavailable", "resume-record-unavailable",
		"frontend-record-unavailable", "frontend-index-mismatch", "frontend-proof-unknown",
		"proof-expired", "bridge-proof-unavailable", "bridge-changed", "pane-changed":
		return reason
	default:
		return "unknown"
	}
}
func holdRotationIdentity(r rotationRequest, reason string, now time.Time) {
	// Expiry belongs to the original request, not the diagnostic clock. It wins
	// even when identity first becomes unavailable shortly before that deadline.
	if now.Unix()-r.CreatedAt >= int64(rotationQueueLimit.Seconds()) {
		failRotation(r, "HQ identity unavailable for reset ("+reason+"); no input sent", now.Unix())
		return
	}
	changed := r.IdentityCause != reason || r.IdentityUnavailableAt == 0 || r.IdentityUnavailableAt > now.Unix()
	if r.IdentityUnavailableAt == 0 || r.IdentityUnavailableAt > now.Unix() {
		r.IdentityUnavailableAt = now.Unix()
	}
	r.IdentityCause = reason
	warn := r.IdentityWarnedAt == 0 && now.Unix()-r.IdentityUnavailableAt >= int64(rotationIdentityWait.Seconds())
	if warn {
		r.IdentityWarnedAt = now.Unix()
	}
	if changed || warn {
		if err := writeRotationFile(r, false); err != nil {
			diag.ReportStoreFailure("hq-rotation", err)
			return
		}
		if changed {
			diag.For("hq").Info("hq.rotation.identity-unavailable", "HQ identity cannot currently be verified; no reset sent", "reason", reason)
		}
	}
	if warn {
		// Persist one attempted publication first. A best-effort queue write (or a
		// crash after it) is never proof of display/read or permission to republish.
		notify.Send(notify.Options{Kind: "attention", Title: i18n.Tr("HQ is waiting for identity verification", "HQ 轮换等待身份确认"), Subtitle: "gtmux", Message: i18n.Tr("HQ identity could not be verified. The request remains pending; no reset was sent.", "暂时无法确认 HQ 身份，轮换请求仍保留，未发送重置命令。"), Pane: r.Pane, Session: "gtmux HQ"})
	}
}
