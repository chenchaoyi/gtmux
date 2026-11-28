package hqnudge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

type acceptedWake struct {
	Pane    string               `json:"pane"`
	Batch   string               `json:"batch"`
	Payload string               `json:"payload"`
	Since   int64                `json:"since"`
	Receipt dispatch.WakeReceipt `json:"receipt"`
}

func (a acceptedWake) standaloneStartup() bool {
	// The original accepted batch carries the producer's leading class and
	// batch suffix. A reference to startup inside another wake is insufficient.
	body, bound := strings.CutSuffix(a.Payload, " · "+a.Batch)
	text, startup := strings.CutPrefix(body, transcript.Sigil+" gtmux·startup │ ")
	return bound && a.Batch != "" && startup && text != "" &&
		!strings.Contains(text, " · ") && !strings.ContainsAny(text, "\r\n")
}

func (a acceptedWake) standaloneWorkspaceTick() bool {
	body, bound := strings.CutSuffix(a.Payload, " · "+a.Batch)
	prefix := hqwake.Line(hqwake.ClassTick, "workspace reconciliation", "pending:")
	fields, tick := strings.CutPrefix(body, prefix)
	if !bound || a.Batch == "" || !tick {
		return false
	}
	count, _, _ := strings.Cut(fields, hqwake.FieldSep())
	pending, err := strconv.Atoi(count)
	return err == nil && pending > 0 &&
		(body == hqwake.WorkspaceReconciliationLine(pending, false) || body == hqwake.WorkspaceReconciliationLine(pending, true))
}

const (
	acceptedProofGrace    = 2 * time.Minute
	acceptedProofLifetime = 30 * time.Minute
)

var wakeWitness = transcript.CodexWakeWitness
var wakeTranscriptHome = transcript.CodexHomeID

func acceptedWakeDir() string { return state.WorkflowPath("hq-wake-accepted") }

func validWakeReceipt(r dispatch.WakeReceipt) bool {
	for _, field := range []string{r.Request, r.Session, r.Home, r.Turn} {
		if field == "" || len(field) > 256 || strings.ContainsAny(field, "\x00\r\n") {
			return false
		}
	}
	return true
}

func readAcceptedWake(path string) (acceptedWake, error) {
	var accepted acceptedWake
	b, err := readNativeBytes(path)
	if err != nil {
		return accepted, err
	}
	if err := json.Unmarshal(b, &accepted); err != nil {
		return accepted, err
	}
	if accepted.Since <= 0 || !validWakeReceipt(accepted.Receipt) {
		return accepted, errors.New("invalid accepted wake receipt")
	}
	return accepted, nil
}

func retainAcceptedWake(r nativeIntent) error {
	dir := acceptedWakeDir()
	name := filepath.Base(nativeIntentPath(r))
	path := filepath.Join(dir, name)
	accepted := acceptedWake{Pane: r.Pane, Batch: r.ID, Payload: r.Payload, Since: r.SubmittedAt, Receipt: *r.WakeReceipt}
	body, err := json.Marshal(accepted)
	if err != nil {
		return err
	}
	previous, err := readNativeBytes(path)
	if err == nil {
		if string(previous) != string(body) {
			return errors.New("accepted wake evidence changed")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(files) >= maxQueueEntries {
		// The receiver already accepted this exact request. Capacity for
		// independent observation is not capacity for input admission.
		return archiveAcceptedWake(name, accepted, "proof-window-full", time.Unix(0, r.SubmittedAt))
	}
	if err := state.WritePrivateFile(path, body); err != nil {
		return err
	}
	diag.For("hq").Act("act.wake.receiver", "system", r.Pane, diag.OK, "HQ receiver acknowledged the wake",
		"batch", r.ID, "receiver_request", r.WakeReceipt.Request, "turn", r.WakeReceipt.Turn)
	return nil
}

// ObserveWakeEvidence does not use the wake queue, current HQ pane, or any
// input operation. Accepted means only the receiver attested admission.
func ObserveWakeEvidence(now time.Time) {
	defer pruneTerminalWakeEvidence(now)
	files, err := os.ReadDir(acceptedWakeDir())
	if errors.Is(err, os.ErrNotExist) || (err == nil && len(files) == 0) {
		return
	}
	if err != nil {
		diag.ReportStoreFailure("hq-wake-evidence", err)
		recordIssue(issueRecoveryUnsafe, now.UnixNano())
		return
	}
	lease, err := state.TryLeaseIn(state.WorkflowPath("hqwake"), "wake-observations")
	if err != nil {
		diag.ReportStoreFailure("hq-wake-evidence", err)
		return
	}
	defer lease.Close()
	home := wakeTranscriptHome()
	const proofBatch = 8
	first := int((now.Unix() / 3) * proofBatch % int64(len(files)))
	if first < 0 {
		first = 0
	}
	for i := 0; i < min(proofBatch, len(files)); i++ {
		file := files[(first+i)%len(files)]
		if strings.HasPrefix(file.Name(), ".private-") {
			continue
		}
		p := filepath.Join(acceptedWakeDir(), file.Name())
		accepted, err := readAcceptedWake(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			recordIssue(issueRecoveryUnsafe, now.UnixNano())
			continue
		}
		since := time.Unix(0, accepted.Since)
		if accepted.Receipt.Home == home {
			reader := wakeWitness
			if now.Sub(since) < acceptedProofLifetime {
				switch {
				case accepted.standaloneStartup():
					reader = transcript.CodexStartupWakeWitness
				case accepted.standaloneWorkspaceTick():
					reader = transcript.CodexFirstTurnWakeWitness
				}
			}
			witness := reader(accepted.Receipt.Session, accepted.Receipt.Turn, accepted.Payload, since)
			if !witness.Started.Before(since) && !witness.Delivered.Before(witness.Started) && !witness.Delivered.After(now) {
				if (!witness.FirstTurnPrefix || rotationSuccessorConfirmed(accepted.Receipt.Session, witness.Started, now)) &&
					confirmAcceptedWake(p, accepted, witness, now) {
					continue
				}
			}
		}
		if now.Sub(since) >= acceptedProofLifetime {
			if err := archiveAcceptedWake(file.Name(), accepted, "proof-window-expired", now); err != nil {
				diag.ReportStoreFailure("hq-wake-evidence", err)
				recordIssue(issueRecoveryUnsafe, now.UnixNano())
				continue
			}
			diag.ReportStoreFailure("hq-wake-evidence", ignoreNativeMissing(os.Remove(p)))
		}
	}
}

// The rotation audit scopes the additional read's result. The receiver and
// contiguous original rollout still supply the session, turn and exact input.
func rotationSuccessorConfirmed(session string, started, now time.Time) bool {
	records := events.Recent(int64(acceptedProofLifetime.Seconds()), now.Unix(), 1, func(r events.Record) bool {
		return r.Event == events.AuditEventRotate && r.AgentKey == "codex" && r.PreviousAgentKey == "codex" &&
			r.Seq > 0 && r.OpID != "" && r.AgentSession == session && r.PreviousAgentSession != "" &&
			r.PreviousAgentSession != session && r.Ts >= started.Unix() && r.Ts <= now.Unix()
	})
	return len(records) == 1
}

// Only an active acknowledgement past its own proof deadline can raise a notice.
// The observer reports invalid evidence separately and archives expired receipts.
func acceptedWakeIssue(now int64) WakeIssue {
	issue := WakeIssue{Reason: issueAcceptedUnverified}
	files, err := os.ReadDir(acceptedWakeDir())
	if err != nil {
		return issue
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".private-") {
			continue
		}
		accepted, err := readAcceptedWake(filepath.Join(acceptedWakeDir(), file.Name()))
		if err != nil {
			continue
		}
		// Round up to the first whole second at or after the proof deadline.
		due := time.Unix(0, accepted.Since).Add(acceptedProofGrace + time.Second - time.Nanosecond).Unix()
		if due > now {
			continue
		}
		if issue.FirstAt == 0 || due < issue.FirstAt {
			issue.FirstAt = due
		}
		issue.LastAt = max(issue.LastAt, due)
	}
	return issue
}

func confirmAcceptedWake(path string, a acceptedWake, witness transcript.WakeWitness, now time.Time) bool {
	recordID := strings.TrimSuffix(filepath.Base(path), ".json")
	appended, err := events.AuditWakeDeliveredChecked(a.Pane, a.Payload, recordID, witness.Delivered.Unix())
	if err != nil {
		diag.ReportStoreFailure("hq-wake-evidence", err)
		return false
	}
	if appended {
		source := "rollout-tail"
		if witness.StartupPrefix {
			source = "startup-prefix"
		} else if witness.FirstTurnPrefix {
			source = "successor-first-turn-prefix"
		}
		diag.For("hq").Act("act.wake.started", "system", a.Pane, diag.OK, "HQ wake turn and prompt independently observed",
			"batch", a.Batch, "receiver_request", a.Receipt.Request, "turn", a.Receipt.Turn, "started_at", witness.Started.UnixNano(), "receipt", recordID, "proof_source", source)
	}
	noteConfirmedDelivery(now.Unix())
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		diag.ReportStoreFailure("hq-wake-evidence", err)
		return false
	}
	return true
}
