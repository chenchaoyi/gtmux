package hqnudge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	stdio "io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
	"golang.org/x/sys/unix"
)

// Intent is durable BEFORE any native call. Neither a crash nor a changed batch
// composition grants permission to repeat a possibly accepted automatic input.
// This is recovery evidence only: it is never used to authorize a new submission.
type nativeIntent struct {
	Version     int                   `json:"version"`
	Pane        string                `json:"pane"`
	ID          string                `json:"id"`
	Payload     string                `json:"payload"`
	Outcome     string                `json:"outcome"`
	Retry       retryState            `json:"retry,omitempty"`
	Reason      string                `json:"reason,omitempty"`
	WakeReceipt *dispatch.WakeReceipt `json:"wake_receipt,omitempty"`
	SubmittedAt int64                 `json:"submitted_at,omitempty"`
	Entries     []nativeEntry         `json:"entries"`
}
type nativeEntry struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	FileID    string `json:"file_id"`
	ChangedAt string `json:"changed_at"`
	Pin       string `json:"pin"`
}

const nativeIntentLimit = 128 << 10

// A valid intent can protect its exact conflicting claim without preventing
// genuinely independent new entries from obtaining their own fresh admission.
var errNativeClaimConflict = errors.New("native wake claim conflict")

func nativeIntentDir() string { return filepath.Join(state.Dir(), "hq-nudge-native-batches") }
func nativeIntentPath(r nativeIntent) string {
	b, _ := json.Marshal(struct {
		ID, Payload string
		Entries     []nativeEntry
	}{r.ID, r.Payload, r.Entries})
	h := sha256.Sum256(b)
	return filepath.Join(nativeIntentDir(), hex.EncodeToString(h[:])+".json")
}
func readNativeBytes(path string) ([]byte, error) {
	f, err := state.OpenPrivateRead(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := stdio.ReadAll(stdio.LimitReader(f, nativeIntentLimit+1))
	if err == nil && len(b) > nativeIntentLimit {
		return nil, errors.New("native wake evidence exceeds budget")
	}
	return b, err
}
func nativeFingerprint(path string) (nativeEntry, error) {
	return nativeFingerprintWithStat(path, unix.Fstat)
}
func nativeFingerprintWithStat(path string, stat func(int, *unix.Stat_t) error) (nativeEntry, error) {
	f, err := state.OpenPrivateRead(path)
	if err != nil {
		return nativeEntry{}, err
	}
	defer f.Close()
	var before, after unix.Stat_t
	if err := stat(int(f.Fd()), &before); err != nil {
		return nativeEntry{}, err
	}
	if before.Ctim.Sec <= 0 || before.Ctim.Nsec < 0 || before.Ctim.Nsec >= 1_000_000_000 {
		return nativeEntry{}, errors.New("native claim generation metadata unavailable")
	}
	b, err := stdio.ReadAll(stdio.LimitReader(f, nativeIntentLimit+1))
	if err != nil {
		return nativeEntry{}, err
	}
	if len(b) > nativeIntentLimit {
		return nativeEntry{}, errors.New("native wake claim exceeds budget")
	}
	if err := stat(int(f.Fd()), &after); err != nil {
		return nativeEntry{}, err
	}
	if before.Dev != after.Dev || before.Ino != after.Ino || before.Size != after.Size || before.Mtim != after.Mtim || before.Ctim != after.Ctim {
		return nativeEntry{}, errors.New("native wake claim changed during read")
	}
	h := sha256.Sum256(b)
	return nativeEntry{Name: filepath.Base(path), SHA256: hex.EncodeToString(h[:]),
		FileID:    fmt.Sprintf("%d:%d", uint64(before.Dev), before.Ino),
		ChangedAt: fmt.Sprintf("%d:%d", before.Ctim.Sec, before.Ctim.Nsec)}, nil
}
func writeNativeIntent(r nativeIntent) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(b) > nativeIntentLimit {
		return errors.New("native wake intent exceeds budget")
	}
	return state.WritePrivateFile(nativeIntentPath(r), b)
}
func prepareNativeIntent(pane, id, payload string, claims []string) (nativeIntent, error) {
	r := nativeIntent{Version: 2, Pane: pane, ID: id, Payload: payload, Outcome: "prepared"}
	if err := admitNativePins(len(claims)); err != nil {
		return r, err
	}
	for _, c := range claims {
		entry, err := pinNativeClaim(r, c)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return r, err
		}
		r.Entries = append(r.Entries, entry)
	}
	if len(r.Entries) == 0 {
		return r, errors.New("native wake claim evidence missing")
	}
	if err := syncNativeDirectory(nativePinDir()); err != nil {
		return r, err
	}
	return r, writeNativeIntent(r)
}
func validNativeIntent(r nativeIntent) bool {
	if r.Version != 2 || r.Pane == "" || len(r.ID) != 7 || r.ID[0] != '#' || strings.Trim(r.ID[1:], "0123456789abcdef") != "" || r.Payload == "" || len(r.Entries) == 0 || len(r.Entries) > maxQueueEntries {
		return false
	}
	switch r.Outcome {
	case "prepared", "uncertain", "accepted", "delivered", "refused", "held":
	default:
		return false
	}
	if r.Retry != (retryState{}) && (r.Outcome != "refused" || r.Retry.Number < 1 ||
		r.Retry.Number >= nativeAttemptLimit || r.Retry.At <= 0 || !validWakeTarget(r.Retry.Target)) {
		return false
	}
	if r.WakeReceipt != nil && (r.Outcome != "accepted" || r.SubmittedAt <= 0 || !validWakeReceipt(*r.WakeReceipt)) {
		return false
	}
	seen := map[string]bool{}
	for _, e := range r.Entries {
		if filepath.Base(e.Name) != e.Name || !strings.HasSuffix(e.Name, sendingSuffix) || !isPendingWake(claimBase(e.Name)) || len(e.SHA256) != 64 || strings.Trim(e.SHA256, "0123456789abcdef") != "" || e.FileID == "" || e.ChangedAt == "" || len(e.Pin) != 68 || !strings.HasSuffix(e.Pin, ".pin") || strings.Trim(strings.TrimSuffix(e.Pin, ".pin"), "0123456789abcdef") != "" || seen[e.Name] {
			return false
		}
		seen[e.Name] = true
	}
	return true
}

// Recovery is serialized by the existing HQ input lease. Prepared means unknown,
// including a stop before the call: retain it, never infer an unsent transaction.
func recoverNativeIntents() error {
	_, err := recoverNativeIntentsWithProtection()
	return err
}
func recoverNativeIntentsWithProtection() (map[string]bool, error) {
	if err := state.MkdirAll(nativeIntentDir(), 0700); err != nil {
		return nil, err
	}
	dir, err := os.Open(nativeIntentDir())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	files, err := dir.ReadDir(maxQueueEntries + 1)
	if err != nil && err != stdio.EOF {
		return nil, err
	}
	if len(files) > maxQueueEntries {
		return nil, errors.New("native wake recovery budget exhausted")
	}
	protected := map[string]bool{}
	var conflicts []error
	for _, e := range files {
		if strings.HasPrefix(e.Name(), ".private-") {
			continue
		} // unpublished private writer temporary
		p := filepath.Join(nativeIntentDir(), e.Name())
		b, err := readNativeBytes(p)
		if err != nil {
			return nil, err
		}
		var r nativeIntent
		if json.Unmarshal(b, &r) != nil || !validNativeIntent(r) || nativeIntentPath(r) != p {
			return nil, errors.New("native wake recovery evidence invalid")
		}
		if err := recoverNativeIntent(r); err != nil {
			if !errors.Is(err, errNativeClaimConflict) {
				return nil, err
			}
			// The record is bounded/valid; its filenames can safely be excluded
			// from ALL orphan reclaim and new claims, without touching the leaf.
			for _, entry := range r.Entries {
				protected[entry.Name] = true
			}
			conflicts = append(conflicts, err)
		} else if r.Outcome == "prepared" {
			r.Outcome = "uncertain"
			r.Reason = "interrupted-attempt"
			recordWakeOutcome(r, time.Now().UnixNano())
		}
	}
	return protected, errors.Join(conflicts...)
}

// A v2 refusal predates both the positive submission boundary and the saved
// target. A matching pane number alone cannot authorize a post-upgrade attempt.
func recoverNativeIntent(r nativeIntent) error {
	if r.Outcome != "refused" || r.Retry.Number != 0 {
		return settleNativeIntent(r)
	}
	for _, entry := range r.Entries {
		if err := validateNativeClaim(r, entry); err != nil && !errors.Is(err, errNativeClaimConflict) {
			return err
		}
		// An interrupted old handoff may have a pending name too. Check its
		// storage before returning a local hold for the entire known slot.
		_, err := nativeFingerprint(claimBase(filepath.Join(queueDir(), entry.Name)))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return fmt.Errorf("%w: legacy refusal lacks verified submission and original target evidence", errNativeClaimConflict)
}

func settleNativeIntent(r nativeIntent) error {
	if r.Outcome == "accepted" && r.WakeReceipt != nil {
		if err := retainAcceptedWake(r); err != nil {
			return err
		}
	}
	// A local conflict may protect an exact slot, but all members must still
	// be checked for unsafe storage before any new submission can proceed.
	var conflicts []error
	var claims []nativeEntry
	for _, e := range r.Entries {
		if err := validateNativeClaim(r, e); err != nil {
			if !errors.Is(err, errNativeClaimConflict) {
				return err
			}
			if r.Outcome == "uncertain" {
				parked, verifyErr := nativeParkedAcrossDeviceChange(e)
				if verifyErr != nil {
					return verifyErr
				}
				if parked {
					continue
				}
			}
			conflicts = append(conflicts, err)
		}
		claims = append(claims, e)
		if r.Outcome == "prepared" || r.Outcome == "uncertain" || r.Outcome == "held" {
			p := filepath.Join(queueDir(), e.Name)
			if err := checkNativeParkDestination(p, finalClaimPath(r, p)); err != nil {
				if !errors.Is(err, errNativeClaimConflict) {
					return err
				}
				conflicts = append(conflicts, err)
			}
		}
	}
	if len(conflicts) > 0 {
		return errors.Join(conflicts...)
	}
	for _, e := range claims {
		p := filepath.Join(queueDir(), e.Name)
		switch r.Outcome {
		case "accepted", "delivered":
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		case "refused":
			if err := returnClaimAt(p, finalClaimPath(r, p)); err != nil {
				return err
			}
		default:
			if err := parkNativeClaimAt(p, finalClaimPath(r, p)); err != nil {
				if !errors.Is(err, errNativeClaimConflict) {
					return err
				}
				conflicts = append(conflicts, err)
			}
		}
	}
	if len(conflicts) > 0 {
		return errors.Join(conflicts...)
	}
	if r.Outcome == "prepared" {
		r.Outcome = "uncertain"
		return writeNativeIntent(r)
	}
	if r.Outcome == "uncertain" {
		return nil
	} // retain exact original payload/delivery identity
	for _, entry := range r.Entries {
		if err := retireNativePin(r, entry); err != nil {
			return err
		}
	}
	if err := syncNativeDirectory(nativePinDir()); err != nil {
		return err
	}
	return ignoreNativeMissing(os.Remove(nativeIntentPath(r)))
}
func validateNativeClaim(r nativeIntent, e nativeEntry) error {
	p := filepath.Join(queueDir(), e.Name)
	observed, claimErr := nativeFingerprint(p)
	if claimErr != nil && !errors.Is(claimErr, os.ErrNotExist) {
		return claimErr
	}
	pin, pinErr := nativeAnchor(e)
	if pinErr != nil && !errors.Is(pinErr, errNativeClaimConflict) {
		return pinErr
	}
	uncertain := r.Outcome == "prepared" || r.Outcome == "uncertain"
	if claimErr != nil && !uncertain {
		// Terminal recovery may have already retired the pin. The terminal
		// cleanup below verifies any extant anchor before removing it.
		return nil
	}
	if pinErr != nil {
		return pinErr
	}
	if uncertain && pin.SHA256 != e.SHA256 {
		return fmt.Errorf("%w: uncertain anchor payload changed", errNativeClaimConflict)
	}
	if claimErr != nil {
		return nil // the uncertain claim is already parked, with its anchor intact
	}
	if observed.FileID != e.FileID {
		return fmt.Errorf("%w: identity no longer matches saved evidence", errNativeClaimConflict)
	}
	if pin.SHA256 != observed.SHA256 {
		return fmt.Errorf("%w: anchor and claim payload differ", errNativeClaimConflict)
	}
	if observed.SHA256 == e.SHA256 && observed.ChangedAt == e.ChangedAt && pin.ChangedAt == observed.ChangedAt {
		return nil
	}
	// Publication changes ctime. Only a recognized shared-inode handoff
	// may account for it; accepted cleanup has no such exception.
	var published string
	if r.Outcome == "refused" || ((uncertain || r.Outcome == "held") && observed.SHA256 == e.SHA256) {
		published = finalClaimPath(r, p)
	}
	if published != "" {
		dst, err := nativeFingerprint(published)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && dst.FileID == observed.FileID {
			return nil
		}
	}
	return fmt.Errorf("%w: saved generation or payload changed", errNativeClaimConflict)
}

func ignoreNativeMissing(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// A hard-link publication is atomic and never replaces another producer's leaf.
// The original claim is removed only after the queue path safely owns its inode.
func finalClaimPath(r nativeIntent, src string) string {
	base := claimBase(src)
	switch r.Outcome {
	case "refused":
		if r.Retry.Number == 0 {
			return base // positively refused v2 intent written before backoff existed
		}
		return filepath.Join(filepath.Dir(src), retryName(filepath.Base(base), r.Retry))
	case "held":
		return base + wakeRefusedSuffix
	default:
		return base + nativeUncertainSuffix
	}
}

func returnClaim(src string) error { return returnClaimAt(src, claimBase(src)) }

func returnClaimAt(src, dst string) error {
	release, err := lockQueue()
	if err != nil {
		return err
	}
	defer release()
	field := queuedKey(dst)
	if field != "" {
		if current := pendingKey(field); current != "" && current != filepath.Base(dst) {
			source, err := nativeFingerprint(src)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if dst != claimBase(src) {
				// Recovery may find the retry link published just before a crash.
				// A newer producer supersedes that exact link as well as the claim.
				published, err := nativeFingerprint(dst)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				if err == nil {
					if published.FileID != source.FileID {
						return errors.New("wake retry publication now refers to different evidence")
					}
					if err := os.Remove(dst); err != nil {
						return err
					}
				}
			}
			auditDroppedFile(events.DropSuperseded, src)
			return os.Remove(src)
		}
	}
	if err := os.Link(src, dst); err == nil {
		return os.Remove(src)
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	a, ea := os.Lstat(src)
	b, eb := os.Lstat(dst)
	if errors.Is(ea, os.ErrNotExist) {
		return nil
	}
	if ea != nil {
		return ea
	}
	if eb != nil {
		return eb
	}
	if os.SameFile(a, b) {
		return os.Remove(src)
	} // recovery after publication, before unlink
	name := filepath.Base(dst)
	if dst == claimBase(src) && strings.Contains(name, ".k-") && b.Mode().IsRegular() {
		auditDroppedFile(events.DropSuperseded, src) // existing latest-keyed contract
		return os.Remove(src)
	}
	return fmt.Errorf("wake return destination already exists: %s", filepath.Base(dst))
}
func parkNativeClaim(src string) error {
	return parkNativeClaimAt(src, claimBase(src)+nativeUncertainSuffix)
}

func parkNativeClaimAt(src, dst string) error {
	if _, err := nativeFingerprint(src); err != nil {
		return ignoreNativeMissing(err)
	}
	if err := os.Link(src, dst); err == nil {
		return os.Remove(src)
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	a, ea := os.Lstat(src)
	b, eb := os.Lstat(dst)
	if errors.Is(ea, os.ErrNotExist) {
		return nil
	}
	if ea != nil {
		return ea
	}
	if eb != nil {
		return eb
	}
	if !os.SameFile(a, b) {
		// Only managed regular files identify an exact conflicting slot. An
		// untrusted or unreadable path is a channel error, even at this location.
		if _, err := nativeFingerprint(src); err != nil {
			return err
		}
		if _, err := nativeFingerprint(dst); err != nil {
			return err
		}
		return fmt.Errorf("%w: native uncertain destination was replaced", errNativeClaimConflict)
	}
	return os.Remove(src)
}

// A recognized occupied slot is local. Any other failure to verify the
// destination prevents admission, including after a preceding local conflict.
func checkNativeParkDestination(src, dst string) error {
	source, err := nativeFingerprint(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	destination, err := nativeFingerprint(dst)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if source.FileID != destination.FileID {
		return fmt.Errorf("%w: native uncertain destination was replaced", errNativeClaimConflict)
	}
	return nil
}
