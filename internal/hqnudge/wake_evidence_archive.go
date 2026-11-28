package hqnudge

import (
	"encoding/json"
	"errors"
	stdio "io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const (
	acceptedArchiveLimit  = 2 << 20
	terminalArchiveLimit  = 2 << 20
	terminalRetainedLimit = 32
	terminalRetainedAge   = 24 * time.Hour
)

// effectiveWakeNowNano ensures age checks against real filesystem ModTimes work
// both when production passes wall-clock nanos and when synthetic tests start an
// injected clock near zero while stamping real wall-clock ModTimes.
func effectiveWakeNowNano(nowNano int64) int64 {
	if wall := time.Now().UnixNano(); wall > nowNano {
		return wall
	}
	return nowNano
}

type acceptedArchiveRecord struct {
	Key      string       `json:"key"`
	Reason   string       `json:"reason"`
	Accepted acceptedWake `json:"accepted"`
}

type terminalArchiveRecord struct {
	ArchivedAt      int64         `json:"archived_at"`
	Kind            string        `json:"kind"`
	Reason          string        `json:"reason,omitempty"`
	Name            string        `json:"name,omitempty"`
	Payload         string        `json:"payload,omitempty"`
	SHA256          string        `json:"sha256,omitempty"`
	FileID          string        `json:"file_id,omitempty"`
	CollisionSHA256 string        `json:"collision_sha256,omitempty"`
	CollisionFileID string        `json:"collision_file_id,omitempty"`
	IntentID        string        `json:"intent_id,omitempty"`
	Pane            string        `json:"pane,omitempty"`
	Entries         []nativeEntry `json:"entries,omitempty"`
}

func acceptedArchivePath() string {
	return state.WorkflowPath("hqwake/accepted-archive.jsonl")
}

func terminalArchivePath() string {
	return state.WorkflowPath("hqwake", "terminal-archive.jsonl")
}

func appendBoundedJSONL(path, leaseName string, limit int64, record any) error {
	lease, err := state.TryLeaseIn(filepath.Dir(path), leaseName)
	if err != nil {
		return err
	}
	defer lease.Close()

	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	line := append(body, '\n')
	f, err := state.OpenPrivateAppend(path)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return errors.Join(err, f.Close())
	}
	if info.Size()+int64(len(line)) > limit {
		if err := f.Close(); err != nil {
			return err
		}
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
		f, err = state.OpenPrivateAppend(path)
		if err != nil {
			return err
		}
	}
	n, err := f.Write(line)
	if err == nil && n != len(line) {
		err = stdio.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	if err = errors.Join(err, f.Close()); err != nil {
		return err
	}
	return syncNativeDirectory(filepath.Dir(path))
}

// Only known receiver acknowledgements may leave the active proof window.
// Both segments are private; archiving never restores the original input.
func archiveAcceptedWake(key string, a acceptedWake, reason string, now time.Time) error {
	path := acceptedArchivePath()
	rec := acceptedArchiveRecord{Key: key, Reason: reason, Accepted: a}
	if err := appendBoundedJSONL(path, "wake-proof-archive", acceptedArchiveLimit, rec); err != nil {
		return err
	}
	recordIssue(issueProofArchived, now.UnixNano())
	diag.For("hq").Act("act.wake.observation.archived", "system", a.Pane, diag.Refused, "Wake receiver acknowledgement archived",
		"batch", a.Batch, "receiver_request", a.Receipt.Request, "turn", a.Receipt.Turn, "reason", reason)
	return nil
}

func appendTerminalArchive(rec terminalArchiveRecord) error {
	return appendBoundedJSONL(terminalArchivePath(), "wake-terminal-archive", terminalArchiveLimit, rec)
}

func archiveSettledNativeIntent(r nativeIntent, reason string, nowNano int64) error {
	rec := terminalArchiveRecord{
		ArchivedAt: nowNano / int64(time.Second),
		Kind:       "native-intent",
		Reason:     reason,
		IntentID:   r.ID,
		Pane:       r.Pane,
		Payload:    r.Payload,
		Entries:    r.Entries,
	}
	if err := appendTerminalArchive(rec); err != nil {
		return err
	}
	for _, entry := range r.Entries {
		if err := ignoreNativeMissing(os.Remove(nativePinPath(entry))); err != nil {
			return err
		}
	}
	if err := syncNativeDirectory(nativePinDir()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return ignoreNativeMissing(os.Remove(nativeIntentPath(r)))
}

// reconcileAgedSlotCollision resolves a historical slot collision where both the
// active claim (src) and a distinct retained terminal file (dst) are regular
// managed files aged past orphanClaimAge. The older retained file and any
// settled uncertain nativeIntent owning its inode are archived first with both
// digests preserved before dst is unlinked.
func reconcileAgedSlotCollision(src, dst string, nowNano int64) (bool, error) {
	nowNano = effectiveWakeNowNano(nowNano)
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return false, ignoreNativeMissing(err)
	}
	dstInfo, err := os.Lstat(dst)
	if err != nil {
		return false, ignoreNativeMissing(err)
	}
	if !srcInfo.Mode().IsRegular() || !dstInfo.Mode().IsRegular() {
		return false, nil
	}
	if os.SameFile(srcInfo, dstInfo) {
		return true, nil
	}
	if nowNano-srcInfo.ModTime().UnixNano() < int64(orphanClaimAge) ||
		nowNano-dstInfo.ModTime().UnixNano() < int64(orphanClaimAge) {
		return false, nil
	}
	srcFP, err := nativeFingerprint(src)
	if err != nil {
		return false, err
	}
	dstFP, err := nativeFingerprint(dst)
	if err != nil {
		return false, err
	}
	dstBody, err := readNativeBytes(dst)
	if err != nil {
		return false, err
	}
	if err := retireMatchingUncertainIntents(dstFP, "colliding-slot-older-intent", nowNano); err != nil {
		return false, err
	}
	rec := terminalArchiveRecord{
		ArchivedAt:      nowNano / int64(time.Second),
		Kind:            "collision",
		Reason:          "colliding-slot-reconciled",
		Name:            filepath.Base(dst),
		Payload:         string(dstBody),
		SHA256:          dstFP.SHA256,
		FileID:          dstFP.FileID,
		CollisionSHA256: srcFP.SHA256,
		CollisionFileID: srcFP.FileID,
	}
	if err := appendTerminalArchive(rec); err != nil {
		return false, err
	}
	if err := ignoreNativeMissing(os.Remove(dst)); err != nil {
		return false, err
	}
	return true, nil
}

func retireMatchingUncertainIntents(target nativeEntry, reason string, nowNano int64) error {
	entries, err := os.ReadDir(nativeIntentDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".private-") {
			continue
		}
		p := filepath.Join(nativeIntentDir(), entry.Name())
		b, err := readNativeBytes(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		var r nativeIntent
		if json.Unmarshal(b, &r) != nil || !validNativeIntent(r) || r.Outcome != "uncertain" {
			continue
		}
		matched := false
		allSettled := true
		for _, item := range r.Entries {
			pin, pinErr := nativeAnchor(item)
			if pinErr != nil || pin.SHA256 != item.SHA256 {
				allSettled = false
				break
			}
			if item.FileID == target.FileID && item.SHA256 == target.SHA256 {
				matched = true
				continue
			}
			claimPath := filepath.Join(queueDir(), item.Name)
			if claimFP, claimErr := nativeFingerprint(claimPath); claimErr == nil && claimFP.FileID == item.FileID {
				allSettled = false
				break
			}
		}
		if matched && allSettled {
			if err := archiveSettledNativeIntent(r, reason, nowNano); err != nil {
				return err
			}
		}
	}
	return nil
}

// pruneTerminalWakeEvidence bounds retained terminal files (.refused,
// .native-uncertain, .legacy-uncertain) and settled uncertain native intents +
// pins so historical failures never exhaust the 200-intent / 1,600-pin budgets.
func pruneTerminalWakeEvidence(now time.Time) {
	lease, err := state.TryLeaseIn(state.WorkflowPath("hqwake"), "wake-terminal-prune")
	if err != nil {
		return
	}
	defer lease.Close()

	nowNano := now.UnixNano()
	queue, err := inspectQueue()
	if err != nil {
		return
	}
	activeSlots := map[string]bool{}
	type retainedItem struct {
		name    string
		base    string
		kind    string
		modNano int64
	}
	var retained []retainedItem
	for _, entry := range queue {
		name := entry.Name()
		switch {
		case strings.HasSuffix(name, sendingSuffix), strings.HasSuffix(name, stuckSuffix):
			activeSlots[claimBase(name)] = true
		case strings.HasSuffix(name, wakeRefusedSuffix):
			info, err := entry.Info()
			if err == nil && info.Mode().IsRegular() {
				retained = append(retained, retainedItem{
					name: name, base: strings.TrimSuffix(name, wakeRefusedSuffix),
					kind: "refused", modNano: info.ModTime().UnixNano(),
				})
			}
		case strings.HasSuffix(name, nativeUncertainSuffix):
			info, err := entry.Info()
			if err == nil && info.Mode().IsRegular() {
				retained = append(retained, retainedItem{
					name: name, base: strings.TrimSuffix(name, nativeUncertainSuffix),
					kind: "native-uncertain", modNano: info.ModTime().UnixNano(),
				})
			}
		case strings.HasSuffix(name, legacyUncertainSuffix):
			info, err := entry.Info()
			if err == nil && info.Mode().IsRegular() {
				retained = append(retained, retainedItem{
					name: name, base: strings.TrimSuffix(name, legacyUncertainSuffix),
					kind: "legacy-uncertain", modNano: info.ModTime().UnixNano(),
				})
			}
		}
	}

	eligible := retained[:0]
	for _, item := range retained {
		if !activeSlots[item.base] {
			eligible = append(eligible, item)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].modNano != eligible[j].modNano {
			return eligible[i].modNano < eligible[j].modNano
		}
		return eligible[i].name < eligible[j].name
	})
	for i, item := range eligible {
		remaining := len(eligible) - i
		aged := nowNano-item.modNano >= int64(terminalRetainedAge)
		if !aged && remaining <= terminalRetainedLimit {
			break
		}
		fullPath := filepath.Join(queueDir(), item.name)
		fp, fpErr := nativeFingerprint(fullPath)
		body, readErr := readNativeBytes(fullPath)
		if fpErr != nil || readErr != nil {
			continue
		}
		reason := "retained-cap"
		if aged {
			reason = "retained-expired"
		}
		if item.kind == "native-uncertain" {
			if err := retireMatchingUncertainIntents(fp, reason, nowNano); err != nil {
				continue
			}
		}
		rec := terminalArchiveRecord{
			ArchivedAt: now.Unix(),
			Kind:       item.kind,
			Reason:     reason,
			Name:       item.name,
			Payload:    string(body),
			SHA256:     fp.SHA256,
			FileID:     fp.FileID,
		}
		if err := appendTerminalArchive(rec); err != nil {
			return
		}
		_ = os.Remove(fullPath)
	}

	intentFiles, err := os.ReadDir(nativeIntentDir())
	if err != nil {
		return
	}
	type settledIntent struct {
		intent nativeIntent
		atNano int64
		name   string
		parked []string
	}
	var settled []settledIntent
	for _, file := range intentFiles {
		if strings.HasPrefix(file.Name(), ".private-") {
			continue
		}
		p := filepath.Join(nativeIntentDir(), file.Name())
		b, err := readNativeBytes(p)
		if err != nil {
			continue
		}
		var r nativeIntent
		if json.Unmarshal(b, &r) != nil || !validNativeIntent(r) || nativeIntentPath(r) != p || r.Outcome != "uncertain" {
			continue
		}
		ok := true
		var parkedPaths []string
		for _, e := range r.Entries {
			base := claimBase(e.Name)
			if activeSlots[base] {
				ok = false
				break
			}
			if _, err := os.Lstat(filepath.Join(queueDir(), e.Name)); !errors.Is(err, os.ErrNotExist) {
				ok = false
				break
			}
			if err := validateNativeClaim(r, e); err != nil {
				ok = false
				break
			}
			parkedPaths = append(parkedPaths, filepath.Join(queueDir(), base+nativeUncertainSuffix))
		}
		if !ok {
			continue
		}
		at := r.SubmittedAt
		if at <= 0 {
			if info, err := file.Info(); err == nil {
				at = info.ModTime().UnixNano()
			}
		}
		settled = append(settled, settledIntent{intent: r, atNano: at, name: file.Name(), parked: parkedPaths})
	}
	sort.Slice(settled, func(i, j int) bool {
		if settled[i].atNano != settled[j].atNano {
			return settled[i].atNano < settled[j].atNano
		}
		return settled[i].name < settled[j].name
	})
	for i, item := range settled {
		remaining := len(settled) - i
		aged := item.atNano > 0 && nowNano-item.atNano >= int64(terminalRetainedAge)
		if !aged && remaining <= terminalRetainedLimit {
			break
		}
		reason := "retained-cap"
		if aged {
			reason = "retained-expired"
		}
		if err := archiveSettledNativeIntent(item.intent, reason, nowNano); err != nil {
			return
		}
		for _, parkedPath := range item.parked {
			fp, fpErr := nativeFingerprint(parkedPath)
			body, readErr := readNativeBytes(parkedPath)
			if fpErr == nil && readErr == nil {
				_ = appendTerminalArchive(terminalArchiveRecord{
					ArchivedAt: now.Unix(),
					Kind:       "native-uncertain",
					Reason:     reason,
					Name:       filepath.Base(parkedPath),
					Payload:    string(body),
					SHA256:     fp.SHA256,
					FileID:     fp.FileID,
				})
				_ = os.Remove(parkedPath)
			}
		}
	}
}
