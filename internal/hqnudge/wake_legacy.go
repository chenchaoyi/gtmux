package hqnudge

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const legacyUncertainSuffix = ".legacy-uncertain"
const enterRepairMaxAttempts = 2
const enterRepairBackoff = 3 * time.Second

type attemptRecord struct {
	ID      string   `json:"id"`
	Payload string   `json:"payload"`
	Since   int64    `json:"since"`
	Entries []string `json:"entries"`
	Pane    string   `json:"pane,omitempty"`
	Target  string   `json:"target,omitempty"`
}

func attemptsDir() string { return state.WorkflowPath("hq-nudge-attempts") }

func legacyAttempt(claims []string, payload, id string, since int64) attemptRecord {
	r := attemptRecord{ID: id, Payload: payload, Since: since}
	for _, path := range claims {
		r.Entries = append(r.Entries, identityOf(claimBase(filepath.Base(path))))
	}
	return r
}

func attemptPath(r attemptRecord, body []byte) string {
	id := strings.TrimPrefix(r.ID, "#")
	if r.Target != "" {
		sum := sha256.Sum256(body)
		id = fmt.Sprintf("%s-%x", id, sum[:12])
	}
	return filepath.Join(attemptsDir(), id+".json")
}

func writeAttempt(r attemptRecord) (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	p := attemptPath(r, b)
	return p, state.WritePrivateFile(p, b)
}

// Retain orphan claims from either terminal transport. Age establishes that a
// process is gone, and says nothing about whether its input reached the receiver.
func holdOrphanClaims(x io, protected map[string]bool) bool {
	nowNano := x.nowNano()
	for _, e := range readQueue() {
		if protected[e.Name()] || !isOrphanClaimAt(e, nowNano) {
			continue
		}
		src := filepath.Join(queueDir(), e.Name())
		dst := claimBase(src) + legacyUncertainSuffix
		err := parkNativeClaimAt(src, dst)
		if errors.Is(err, errNativeClaimConflict) {
			if ok, recErr := reconcileAgedSlotCollision(src, dst, nowNano); recErr != nil {
				err = recErr
			} else if ok {
				err = parkNativeClaimAt(src, dst)
			}
		}
		if err != nil {
			diag.ReportStoreFailure("hq-wake-queue", err)
			if !errors.Is(err, errNativeClaimConflict) {
				recordIssue(issueRecoveryUnsafe, nowNano)
				return false
			}
			protected[e.Name()] = true
			recordIssue(issueClaimConflict, nowNano)
			continue
		}
		recordIssue(issueUnknownLegacyInput, nowNano)
		wakeStage("act.wake.outcome.unknown", claimBase(src), "", diag.Failed, "orphaned-claim")
	}
	return true
}

func wakeBusy(x io, pane string) bool {
	return x.busy != nil && x.busy(pane)
}

func readyLegacyTarget(x io, pane, target string) bool {
	return validWakeTarget(target) && wakeIdentity(x, pane) == target &&
		!wakeBusy(x, pane) && (x.inMode == nil || !x.inMode(pane))
}

func attemptLegacyWake(x io, pane, payload, id string, claims []string) {
	r := legacyAttempt(claims, payload, id, x.nowNano()/int64(time.Second)-1)
	r.Pane, r.Target = pane, wakeIdentity(x, pane)
	if !readyLegacyTarget(x, pane, r.Target) || !boxEmpty(x, pane) {
		for _, claim := range claims {
			diag.ReportStoreFailure("hq-wake-queue", returnClaim(claim))
		}
		return
	}
	path, err := writeAttempt(r)
	if err != nil {
		diag.ReportStoreFailure("hq-wake-attempt", err)
		recordIssue(issueRecoveryUnsafe, x.nowNano())
		return
	}
	switch deliverPayload(x, pane, r) {
	case delivered:
		finishDelivered(pane, payload, claims)
		diag.ReportStoreFailure("hq-wake-attempt", ignoreNativeMissing(os.Remove(path)))
	case unsubmitted:
		m := repairState{attemptRecord: r, RecordPath: filepath.Base(path), NextAt: x.nowNano() + int64(enterRepairBackoff)}
		for _, c := range claims {
			if err := parkNativeClaimAt(c, claimBase(c)+stuckSuffix); err != nil {
				diag.ReportStoreFailure("hq-wake-attempt", err)
				recordIssue(issueRecoveryUnsafe, x.nowNano())
				return
			}
		}
		if err := writeRepair(m); err != nil {
			diag.ReportStoreFailure("hq-wake-attempt", err)
			recordIssue(issueRecoveryUnsafe, x.nowNano())
		}
	default:
		holdLegacyWake(x, r, claims)
	}
}

type outcome int

const (
	delivered outcome = iota
	unacked
	unsubmitted
)

func deliverPayload(x io, pane string, r attemptRecord) outcome {
	target, payload, id := r.Target, r.Payload, r.ID
	if !readyLegacyTarget(x, pane, target) || !boxEmpty(x, pane) ||
		!readyLegacyTarget(x, pane, target) || x.paste(pane, payload) != nil {
		return unacked
	}
	diag.For("hq").Act("act.wake.pasted", "system", pane, diag.OK, "Terminal paste returned success", "batch", id)
	if exactWakeDraft(x, pane, target, payload) {
		diag.For("hq").Act("act.wake.entered", "system", pane, diag.OK, "Exact wake draft reached composer", "batch", id)
		_ = x.enter(pane)
		x.sleep()
	}
	if wakeIdentity(x, pane) != target {
		return unacked
	}
	if ackConfirmed(x, pane, id, r.Since) {
		return delivered
	}
	if exactWakeDraft(x, pane, target, payload) {
		return unsubmitted
	}
	return unacked
}

func exactWakeDraft(x io, pane, target, payload string) bool {
	capture := x.captureColor
	if capture == nil {
		capture = x.capture
	}
	for frame := 0; frame < 2; frame++ {
		if !readyLegacyTarget(x, pane, target) {
			return false
		}
		draft, structured := dispatch.DraftOfColored(capture(pane))
		if !structured || !dispatch.DraftIsExactly(draft, payload) {
			return false
		}
		if frame == 0 {
			x.sleep()
		}
	}
	return readyLegacyTarget(x, pane, target)
}

func holdLegacyWake(x io, r attemptRecord, claims []string) {
	for _, claim := range claims {
		base := claimBase(claim)
		if err := parkNativeClaimAt(claim, base+legacyUncertainSuffix); err != nil {
			diag.ReportStoreFailure("hq-wake-attempt", err)
			recordIssue(issueRecoveryUnsafe, x.nowNano())
		}
		wakeStage("act.wake.outcome.unknown", base, r.ID, diag.Failed, "terminal-ack-missing")
	}
	recordIssue(issueUnknownLegacyInput, x.nowNano())
	writeFailCount(readFailCount() + 1)
}

type repairState struct {
	attemptRecord
	RecordPath string `json:"record_path,omitempty"`
	Attempts   int    `json:"attempts"`
	NextAt     int64  `json:"next_at,omitempty"`
	InFlight   bool   `json:"in_flight,omitempty"`
}

func repairPath() string { return filepath.Join(queueDir(), "enter-repair") }

func readRepair() (repairState, bool) {
	var m repairState
	b, err := readNativeBytes(repairPath())
	if err != nil || json.Unmarshal(b, &m) != nil || !validLegacyAttempt(m.attemptRecord) {
		return repairState{}, false
	}
	return m, true
}

func writeRepair(m repairState) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return state.WritePrivateFile(repairPath(), b)
}

func validLegacyAttempt(r attemptRecord) bool {
	return len(r.ID) == 7 && strings.HasPrefix(r.ID, "#") &&
		strings.Trim(r.ID[1:], "0123456789abcdef") == "" && r.Payload != ""
}

func repairClaims(m repairState) []string {
	var paths []string
	for _, entry := range readQueue() {
		if !strings.HasSuffix(entry.Name(), stuckSuffix) {
			continue
		}
		if len(m.Entries) == 0 || slices.Contains(m.Entries, identityOf(claimBase(entry.Name()))) {
			paths = append(paths, filepath.Join(queueDir(), entry.Name()))
		}
	}
	return paths
}

func repairStranded(x io, pane string) {
	m, ok := readRepair()
	if !ok {
		return
	}
	claims := repairClaims(m)
	targetMatches := m.Pane == pane && validWakeTarget(m.Target) && wakeIdentity(x, pane) == m.Target
	pathValid := m.RecordPath != "" && filepath.Base(m.RecordPath) == m.RecordPath
	if targetMatches && ackConfirmed(x, pane, m.ID, m.Since) {
		finishDelivered(pane, m.Payload, claims)
		if pathValid {
			diag.ReportStoreFailure("hq-wake-attempt", ignoreNativeMissing(os.Remove(filepath.Join(attemptsDir(), m.RecordPath))))
		}
		diag.ReportStoreFailure("hq-wake-attempt", ignoreNativeMissing(os.Remove(repairPath())))
		return
	}
	if targetMatches && !m.InFlight && len(claims) > 0 && m.Attempts < enterRepairMaxAttempts {
		window, err := hqinput.RotationWindow(pane, x.nowNano()/int64(time.Second))
		if err != nil || window == hqinput.RotationHandoff {
			return
		}
		if !readyLegacyTarget(x, pane, m.Target) {
			return
		}
		if exactWakeDraft(x, pane, m.Target, m.Payload) {
			if x.nowNano() < m.NextAt {
				return
			}
			m.InFlight = true
			m.Attempts++
			if err := writeRepair(m); err != nil {
				diag.ReportStoreFailure("hq-wake-attempt", err)
				recordIssue(issueRecoveryUnsafe, x.nowNano())
				return
			}
			if exactWakeDraft(x, pane, m.Target, m.Payload) {
				_ = x.enter(pane)
				x.sleep()
				if ackConfirmed(x, pane, m.ID, m.Since) && wakeIdentity(x, pane) == m.Target {
					repairStranded(x, pane)
					return
				}
				if m.Attempts < enterRepairMaxAttempts && exactWakeDraft(x, pane, m.Target, m.Payload) {
					m.InFlight = false
					m.NextAt = x.nowNano() + int64(enterRepairBackoff)
					diag.ReportStoreFailure("hq-wake-attempt", writeRepair(m))
					return
				}
			}
		}
	}
	holdLegacyWake(x, m.attemptRecord, claims)
	diag.ReportStoreFailure("hq-wake-attempt", ignoreNativeMissing(os.Remove(repairPath())))
}

// Late receipts can close only their original entries. The legacy same-name
// slot protection is applied before any rename, including during an upgrade.
func settleEarlierAttempts(x io, pane string, protected map[string]bool) bool {
	files, err := os.ReadDir(attemptsDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		diag.ReportStoreFailure("hq-wake-attempt", err)
		recordIssue(issueRecoveryUnsafe, x.nowNano())
		return false
	}
	clean := true
	queue := readQueue()
	target := wakeIdentity(x, pane)
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".private-") {
			continue
		}
		p := filepath.Join(attemptsDir(), file.Name())
		var r attemptRecord
		b, err := readNativeBytes(p)
		if err != nil || json.Unmarshal(b, &r) != nil || !validLegacyAttempt(r) {
			recordIssue(issueRecoveryUnsafe, x.nowNano())
			clean = false
			continue
		}
		if r.Target != "" && (r.Pane != pane || r.Target != target) {
			continue
		}
		members := legacyAttemptMembers(r, queue)
		if len(members) == 0 {
			// Cleanup only an attempt that has no matching source in any phase
			// and has outlived a drainer, including the previous on-disk format.
			if !legacyAttemptOwnsSource(r, queue) && x.nowNano()/int64(time.Second)-r.Since > int64(orphanClaimAge.Seconds()) {
				diag.ReportStoreFailure("hq-wake-attempt", ignoreNativeMissing(os.Remove(p)))
			}
			continue
		}
		if !ackConfirmed(x, pane, r.ID, r.Since) {
			continue
		}
		var claimed []string
		for _, src := range members {
			base := strings.TrimSuffix(src, legacyUncertainSuffix)
			if protected[filepath.Base(base)+sendingSuffix] {
				continue
			}
			dst := base + sendingSuffix
			if takeQueued(src, dst) {
				claimed = append(claimed, dst)
			}
		}
		if len(claimed) > 0 {
			finishDelivered(pane, r.Payload, claimed)
		}
		if len(claimed) == len(members) {
			diag.ReportStoreFailure("hq-wake-attempt", ignoreNativeMissing(os.Remove(p)))
		}
	}
	return clean
}

func legacyAttemptMembers(r attemptRecord, queue []os.DirEntry) []string {
	var paths []string
	for _, entry := range queue {
		name := entry.Name()
		base, parked := strings.CutSuffix(name, legacyUncertainSuffix)
		if !isPendingWake(base) || isRelayAttention(base) || parked != (r.Target != "") {
			continue
		}
		if !parked && attemptsOf(base) == 0 {
			continue // a reused pre-upgrade name cannot prove ownership
		}
		if slices.Contains(r.Entries, identityOf(base)) {
			paths = append(paths, filepath.Join(queueDir(), name))
		}
	}
	return paths
}

func legacyAttemptOwnsSource(r attemptRecord, queue []os.DirEntry) bool {
	for _, entry := range queue {
		name := strings.TrimSuffix(entry.Name(), legacyUncertainSuffix)
		name = strings.TrimSuffix(name, nativeUncertainSuffix)
		if slices.Contains(r.Entries, identityOf(claimBase(name))) {
			return true
		}
	}
	return false
}
