package hqnudge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const (
	issueClaimConflict  = "claim-conflict"
	issueRecoveryUnsafe = "recovery-evidence"
	issueQueuePressure  = "queue-pressure"
	issueQueueIO        = "queue-io"
	issueUnknownInput   = "submission-unknown"
)

var issueCodes = []string{
	issueClaimConflict, issueRecoveryUnsafe, issueQueuePressure, issueQueueIO, issueUnknownInput,
}

type WakeIssue struct {
	Reason  string
	FirstAt int64
	LastAt  int64
}

type Health struct {
	Queued    int
	Uncertain int
	Issues    []WakeIssue
}

func wakeHealthPath(name string) string {
	return filepath.Join(state.Dir(), "hqwake", "wake-health-"+name)
}

func readIssue(reason string) (WakeIssue, error) {
	var issue WakeIssue
	b, err := readNativeBytes(wakeHealthPath(reason))
	if err != nil {
		return issue, err
	}
	if err := json.Unmarshal(b, &issue); err != nil || issue.Reason != reason || issue.FirstAt <= 0 || issue.LastAt < issue.FirstAt {
		return WakeIssue{}, errors.New("invalid wake health marker")
	}
	return issue, nil
}

// The same reason may be seen by every fast tick. Persist the first observation
// and at most one timestamp per minute thereafter; receipt evidence is separate.
func recordIssue(reason string, nowNano int64) {
	now := nowNano / int64(time.Second)
	if now <= 0 {
		return
	}
	lease, err := state.TryLeaseIn(filepath.Dir(wakeHealthPath(reason)), "wake-health-"+reason)
	if err != nil {
		diag.ReportStoreFailure("hq-wake-health", err)
		return
	}
	defer lease.Close()
	issue, err := readIssue(reason)
	if err == nil && now-issue.LastAt < 60 {
		return
	}
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			diag.ReportStoreFailure("hq-wake-health", err)
			return
		}
		issue = WakeIssue{Reason: reason, FirstAt: now}
	}
	issue.LastAt = now
	data, err := json.Marshal(issue)
	if err == nil {
		err = state.WritePrivateFile(wakeHealthPath(reason), data)
	}
	diag.ReportStoreFailure("hq-wake-health", err)
}

func noteCleanRecovery(nowNano int64) {
	var last int64
	for _, reason := range []string{issueClaimConflict, issueRecoveryUnsafe} {
		if issue, err := readIssue(reason); err == nil && issue.LastAt > last {
			last = issue.LastAt
		}
	}
	p := wakeHealthPath("recovery-scanned")
	if last > 0 && state.ReadInt64Marker(p) <= last {
		diag.ReportStoreFailure("hq-wake-health", state.WriteInt64Marker(p, nowNano/int64(time.Second)))
	}
}

func noteConfirmedDelivery(now int64) {
	diag.ReportStoreFailure("hq-wake-health", state.WriteInt64Marker(wakeHealthPath("confirmed"), now))
}

// ReadHealth does not mutate queue or alert state. Old uncertain batches stay
// visible as inventory without permanently redefining the live pending backlog.
func ReadHealth(now int64) Health {
	var out Health
	var oldest int64
	for _, entry := range readQueue() {
		name := entry.Name()
		if strings.HasSuffix(name, nativeUncertainSuffix) {
			out.Uncertain++
		}
		if !strings.HasSuffix(name, ".txt") {
			continue
		}
		out.Queued++
		if dueOf(name) > now*int64(time.Second) {
			continue
		}
		info, err := entry.Info()
		if err == nil && (oldest == 0 || info.ModTime().Unix() < oldest) {
			oldest = info.ModTime().Unix()
		}
	}
	confirmed := state.ReadInt64Marker(wakeHealthPath("confirmed"))
	recovery := state.ReadInt64Marker(wakeHealthPath("recovery-scanned"))
	for _, reason := range issueCodes {
		i, err := readIssue(reason)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			out.Issues = append(out.Issues, WakeIssue{Reason: issueRecoveryUnsafe, LastAt: now})
			continue
		}
		clear := confirmed > i.LastAt
		switch reason {
		case issueClaimConflict, issueRecoveryUnsafe:
			clear = clear && recovery > i.LastAt
		case issueQueuePressure:
			clear = clear && out.Queued < maxQueueEntries
		case issueUnknownInput:
			// A success for another batch cannot settle this submitted-unknown evidence.
			clear = false
		}
		if !clear {
			out.Issues = append(out.Issues, i)
		}
	}
	if oldest > 0 && now-oldest > staleQueueSecs {
		out.Issues = append(out.Issues, WakeIssue{Reason: "stalled", FirstAt: oldest, LastAt: now})
	}
	if readFailCount() >= wakeFailLimit {
		out.Issues = append(out.Issues, WakeIssue{Reason: "repeated-failure", LastAt: now})
	}
	return out
}
