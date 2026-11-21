package hqnudge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
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

const acceptedProofGrace = 2 * time.Minute

var wakeWitness = transcript.CodexWakeWitness
var wakeTranscriptHome = transcript.CodexHomeID

func acceptedWakeDir() string { return filepath.Join(state.Dir(), "hq-wake-accepted") }

func validWakeReceipt(r dispatch.WakeReceipt) bool {
	for _, field := range []string{r.Request, r.Session, r.Home, r.Turn} {
		if field == "" || len(field) > 256 || strings.ContainsAny(field, "\x00\r\n") {
			return false
		}
	}
	return true
}

func retainAcceptedWake(r nativeIntent) error {
	dir := acceptedWakeDir()
	name := filepath.Base(nativeIntentPath(r))
	path := filepath.Join(dir, name)
	body, err := json.Marshal(acceptedWake{Pane: r.Pane, Batch: r.ID, Payload: r.Payload, Since: r.SubmittedAt, Receipt: *r.WakeReceipt})
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
		return errors.New("accepted wake proof budget exhausted")
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
	files, err := os.ReadDir(acceptedWakeDir())
	if errors.Is(err, os.ErrNotExist) || (err == nil && len(files) == 0) {
		return
	}
	if err != nil {
		diag.ReportStoreFailure("hq-wake-evidence", err)
		recordIssue(issueRecoveryUnsafe, now.UnixNano())
		return
	}
	lease, err := state.TryLeaseIn(filepath.Join(state.Dir(), "hqwake"), "wake-observations")
	if err != nil {
		diag.ReportStoreFailure("hq-wake-evidence", err)
		return
	}
	defer lease.Close()
	home := wakeTranscriptHome()
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".private-") {
			continue
		}
		p := filepath.Join(acceptedWakeDir(), file.Name())
		var accepted acceptedWake
		b, err := readNativeBytes(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || json.Unmarshal(b, &accepted) != nil || accepted.Since <= 0 ||
			!validWakeReceipt(accepted.Receipt) {
			recordIssue(issueRecoveryUnsafe, now.UnixNano())
			continue
		}
		since := time.Unix(0, accepted.Since)
		if accepted.Receipt.Home == home {
			witness := wakeWitness(accepted.Receipt.Session, accepted.Receipt.Turn, accepted.Payload, since)
			if !witness.Started.Before(since) && !witness.Delivered.Before(witness.Started) && !witness.Delivered.After(now) {
				if confirmAcceptedWake(p, accepted, witness, now) {
					continue
				}
			}
		}
		if now.Sub(since) >= acceptedProofGrace {
			recordIssue(issueAcceptedUnverified, now.UnixNano())
		}
	}
}

func confirmAcceptedWake(path string, a acceptedWake, witness transcript.WakeWitness, now time.Time) bool {
	recordID := strings.TrimSuffix(filepath.Base(path), ".json")
	if err := events.AuditWakeDeliveredChecked(a.Pane, a.Payload, recordID, witness.Delivered.Unix()); err != nil {
		diag.ReportStoreFailure("hq-wake-evidence", err)
		return false
	}
	diag.For("hq").Act("act.wake.started", "system", a.Pane, diag.OK, "HQ wake turn and prompt independently observed",
		"batch", a.Batch, "receiver_request", a.Receipt.Request, "turn", a.Receipt.Turn, "started_at", witness.Started.UnixNano(), "receipt", recordID)
	noteConfirmedDelivery(now.Unix())
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		diag.ReportStoreFailure("hq-wake-evidence", err)
		return false
	}
	return true
}
