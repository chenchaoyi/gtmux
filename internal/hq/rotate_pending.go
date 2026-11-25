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

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/resume"
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
	BootstrapID           string `json:"bootstrapID,omitempty"`
	BootstrapHome         string `json:"bootstrapHome,omitempty"`
	BootstrapInstance     string `json:"bootstrapInstance,omitempty"`
	BootstrapPID          int    `json:"bootstrapPID,omitempty"`
	Pane                  string `json:"pane"`
	Agent                 string `json:"agent"`
	Retiring              string `json:"retiring"`
	Input                 string `json:"input"`
	CreatedAt             int64  `json:"createdAt"`
	SentAt                int64  `json:"sentAt,omitempty"`
	SubmittedAtNS         int64  `json:"submittedAtNS,omitempty"`
	ReserveBrief          bool   `json:"reserveBrief,omitempty"`
}

const (
	rotationReadyDelay   = 3 * time.Second
	rotationSettleWait   = hqinput.RotationSettleWait
	rotationQueueLimit   = 30 * time.Minute
	rotationReadyWait    = hqinput.RotationReadyWait
	rotationIdentityWait = time.Minute
)

// Use the existing guarded HQ startup channel, after releasing the shared input
// lock. This is the rotation's handoff, not an invented user task or a second /new.
var rotationBootstrap = func(brief hqinput.RotationBrief) {
	hqnudge.DeliverRotationBrief(brief, hqBriefingPrompt())
}

var rotationBriefFrontend = resume.CodexFrontendForPane

var rotationTargetStamp = tmux.PaneGeneration

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

func rotationPendingPath() string { return state.WorkflowPath("hqwake", "rotate-pending.json") }

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
	if !rotationTargetMatches(r.TargetStamp, r.TargetStamp) {
		failQueuedRotation(r, "HQ target changed or its pre-reboot identity is unavailable; no input sent", time.Now().Unix())
		return
	}
	agent, sid, reason := hqSessionEvidence(pane, &r)
	if reason == "agent-driver-unavailable" {
		agent, sid = "", "" // a historical direct-client record alone is not live evidence
	}
	if pane == r.Pane && r.SentAt == 0 && sid != "" && sid != r.Retiring {
		// A sample from the unsent phase cannot settle a concurrent reset.
		failQueuedRotation(r, "HQ session changed before reset was sent", time.Now().Unix())
		return
	}
	advanceHQRotationWithClock(r, pane, agent, sid, dispatchbridge.DispatchIO(pane), transcript.CodexLastTurnBoundary, time.Now, reason)
}

func advanceHQRotationWithClock(r rotationRequest, pane, agent, sid string, io dispatch.IO,
	boundary func(string) (string, time.Time), clock func() time.Time, identityReason ...string) {
	now := clock()
	release, acquired, err := hqinput.Acquire()
	if err != nil {
		diag.ReportStoreFailure("hq-input", err)
		return
	}
	if !acquired {
		return // keep the durable request while another HQ input channel is active
	}
	var bootstrap hqinput.RotationBrief
	defer func() {
		release()
		if bootstrap.Request != "" && clock().Unix() < bootstrap.Deadline {
			rotationBootstrap(bootstrap)
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
	if pane != r.Pane || agent != "" && agent != r.Agent {
		failRotation(r, "HQ pane or agent changed", now.Unix())
		return
	}
	if r.TargetStamp != "" {
		stamp := rotationTargetStamp(pane)
		if stamp == "" && r.SentAt != 0 {
			if now.Unix() >= rotationDeadline(r) {
				failSentRotationTimeout(r, "", "", "pane-generation-unavailable", now.Unix())
			}
			return
		}
		if !rotationTargetMatches(r.TargetStamp, stamp) {
			failRotation(r, "HQ target changed while waiting for input; no input sent", now.Unix())
			return
		}
	}
	reason := ""
	if len(identityReason) > 0 {
		reason = rotationIdentityCode(identityReason[0])
	}
	if r.SentAt != 0 {
		deadline := rotationDeadline(r)
		if r.SubmittedAtNS <= 0 {
			if now.Unix() >= deadline {
				failRotation(r, "HQ reset key completion was not recorded; no reset will be retried", now.Unix())
			}
			return
		}
		// Lifecycle events carry neither a pane generation nor send order within
		// a second. Only the current pane's live evidence can establish a successor.
		if agent != "" && sid != "" && sid != r.Retiring {
			if r.Successor != "" && sid != r.Successor {
				failRotation(r, "HQ changed again before its successor became recoverable", now.Unix())
				return
			}
			kind := ""
			var boundaryAt time.Time
			ready := r.Agent != "codex"
			if !ready {
				kind, boundaryAt = boundary(sid)
				ready = transcript.CodexRecovery(sid).State == transcript.RecoveryReady && kind != "" &&
					boundaryAt.UnixNano() >= r.SubmittedAtNS && !boundaryAt.After(time.Unix(deadline, 0))
			}
			checkedAt := clock()
			if ready && !boundaryAt.After(checkedAt) {
				events.AuditRotateConfirmed(r.Agent, r.Retiring, sid, r.Input, checkedAt.Unix())
				_ = os.Remove(rotationPendingPath())
				return
			}
			if checkedAt.Unix() >= deadline {
				failSentRotationTimeout(r, agent, sid, reason, checkedAt.Unix())
				return
			}
			if r.Successor == "" {
				r.Successor, r.SwitchedAt = sid, now.Unix()
				deadline = rotationDeadline(r)
				if err := writeRotationFile(r, false); err != nil {
					diag.ReportStoreFailure("hq-rotation", err)
					return
				}
				diag.For("hq").Info("hq.rotation.switched", "HQ successor observed; waiting for durable handoff", "session", sid)
			}
			if r.BootstrapAt == 0 && kind == "" && hqBriefingEnabled() &&
				!state.Exists(state.ActivePath(pane)) && !state.Exists(state.WaitingPath(pane)) && dispatch.BoxEmpty(io) {
				frontend, bound := rotationBriefFrontend(pane)
				if !bound || frontend.SessionID != sid || frontend.HomeID != transcript.CodexHomeID() ||
					frontend.Instance == "" || frontend.PID <= 0 {
					return
				}
				bootstrapAt := clock().Unix()
				if bootstrapAt >= deadline {
					failSentRotationTimeout(r, agent, sid, reason, bootstrapAt)
					return
				}
				r.BootstrapAt = bootstrapAt
				r.BootstrapID, r.BootstrapHome = diag.NewOpID(), frontend.HomeID
				r.BootstrapInstance, r.BootstrapPID = frontend.Instance, frontend.PID
				if err := writeRotationFile(r, false); err != nil {
					diag.ReportStoreFailure("hq-rotation", err)
					return
				}
				bootstrap = hqinput.RotationBrief{
					Request: r.BootstrapID, Pane: r.Pane, Agent: r.Agent, Home: r.BootstrapHome,
					Session: r.Successor, Stamp: r.TargetStamp, Instance: r.BootstrapInstance,
					ClientPID: r.BootstrapPID, Deadline: deadline,
				}
			}
			return
		}
		if now.Unix() >= deadline {
			failSentRotationTimeout(r, agent, sid, reason, now.Unix())
		}
		return
	}
	if sid != "" && sid != r.Retiring {
		failRotation(r, "HQ session changed before reset was sent", now.Unix())
		return
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
	r.SentAt = clock().Unix()
	r.ReserveBrief = r.Agent == "codex" && hqBriefingEnabled()
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
		return
	}
	// A completed key call gives legacy witnesses an ordering fence, separate
	// from the attempt which already prevents retry. A crash here keeps the
	// attempt recorded and leaves legacy evidence ineligible.
	r.SubmittedAtNS = clock().UnixNano()
	diag.ReportStoreFailure("hq-rotation", writeRotationFile(r, false))
}

func rotationDeadline(r rotationRequest) int64 {
	if r.SwitchedAt != 0 {
		return r.SwitchedAt + int64(rotationReadyWait/time.Second)
	}
	return r.SentAt + int64(rotationSettleWait/time.Second)
}

func legacyRotationObservation(proof resume.HQObservation, r rotationRequest, now time.Time) bool {
	return r.SentAt > 0 && r.SubmittedAtNS > 0 && proof.ClientPID > 1 && proof.ClientBirth != "" &&
		rotationTargetMatches(r.TargetStamp, proof.Generation) &&
		proof.ObservedAtNS > r.SubmittedAtNS && proof.ObservedAtNS <= now.UnixNano() &&
		proof.ObservedAtNS <= time.Unix(rotationDeadline(r), 0).UnixNano()
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
	queued := wakeDesktopNotice(notify.Options{
		Kind: "attention", Title: i18n.Tr("HQ handoff did not complete", "HQ 轮换未完成"),
		Subtitle: "gtmux", Message: reason, Pane: r.Pane, Session: "gtmux HQ",
	})
	diag.For("hq").Info("hq.rotation.failure_notice", "HQ handoff failure journaled",
		"desktop_queued", queued, "reset_attempted", r.SentAt != 0)
	diag.ReportStoreFailure("hq-rotation", os.Remove(rotationPendingPath()))
}

func failSentRotationTimeout(r rotationRequest, agent, sid, code string, now int64) {
	reason := "reset was attempted but no new session ID appeared"
	if r.Successor != "" {
		reason = "HQ successor was observed but its handoff/history was not confirmed"
	} else if agent != "" && sid != "" && sid != r.Retiring {
		reason = "HQ successor was observed without recoverable history at the settle deadline"
	}
	switch {
	case code == "pane-generation-unavailable":
		reason += " (HQ pane generation unavailable)"
	case agent == "":
		if code == "" {
			code = "agent-evidence-unavailable"
		}
		reason += " (HQ agent identity unavailable: " + code + ")"
	case sid == "":
		if code == "" {
			code = "session-evidence-unavailable"
		}
		reason += " (HQ session identity unavailable: " + code + ")"
	}
	failRotation(r, reason+"; no reset will be retried", now)
}

// Only known, value-free observation codes enter persisted diagnostics or logs.
func rotationIdentityCode(reason string) string {
	switch reason {
	case "", "pane-location-unavailable", "agent-driver-unavailable", "resume-record-unavailable",
		"frontend-record-unavailable", "frontend-index-mismatch", "frontend-proof-unknown",
		"proof-expired", "bridge-proof-unavailable", "bridge-changed", "pane-proof-unavailable", "pane-changed",
		"backend-proof-unavailable", "backend-changed", "terminal-proof-unavailable", "terminal-changed", "frontend-retired", "legacy-pane-proof-unavailable":
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
