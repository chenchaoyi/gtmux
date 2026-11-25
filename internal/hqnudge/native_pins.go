package hqnudge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	stdio "io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const nativePinLimit = maxQueueEntries * maxBatchLines

func nativePinDir() string               { return state.WorkflowPath("hq-nudge-native-claims") }
func nativePinPath(e nativeEntry) string { return filepath.Join(nativePinDir(), e.Pin) }

// A live link prevents natural inode reuse even on coarse timestamp filesystems.
// It is protocol evidence under the same cooperating-writer lease, not a lock
// against a same-user adversary replacing every pin and journal.
func admitNativePins(count int) error {
	if err := state.MkdirAll(nativePinDir(), 0700); err != nil {
		return err
	}
	f, err := os.Open(nativePinDir())
	if err != nil {
		return err
	}
	defer f.Close()
	files, err := f.ReadDir(nativePinLimit + 1)
	if err != nil && err != stdio.EOF {
		return err
	}
	if len(files)+count > nativePinLimit {
		return errors.New("native claim anchor budget exhausted")
	}
	return nil
}
func pinNativeClaim(r nativeIntent, src string) (nativeEntry, error) {
	before, err := nativeFingerprint(src)
	if err != nil {
		return nativeEntry{}, err
	}
	h := sha256.Sum256([]byte(r.Pane + "\x00" + r.ID + "\x00" + before.Name + "\x00" + before.FileID + "\x00" + before.SHA256))
	name := hex.EncodeToString(h[:]) + ".pin"
	path := filepath.Join(nativePinDir(), name)
	if err := os.Link(src, path); err != nil && !errors.Is(err, os.ErrExist) {
		return nativeEntry{}, err
	}
	source, err := nativeFingerprint(src)
	if err != nil {
		return nativeEntry{}, err
	}
	pin, err := nativeFingerprint(path)
	if err != nil {
		return nativeEntry{}, err
	}
	if source.FileID != before.FileID || source.SHA256 != before.SHA256 || pin.FileID != source.FileID || pin.SHA256 != source.SHA256 || pin.ChangedAt != source.ChangedAt {
		return nativeEntry{}, errors.New("native anchor source changed before intent publication")
	}
	source.Pin = name
	return source, nil
}
func syncNativeDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func nativeAnchor(e nativeEntry) (nativeEntry, error) {
	pin, err := nativeFingerprint(nativePinPath(e))
	if errors.Is(err, os.ErrNotExist) {
		return nativeEntry{}, fmt.Errorf("%w: anchor missing", errNativeClaimConflict)
	}
	if err != nil {
		return nativeEntry{}, err
	}
	if pin.FileID != e.FileID {
		return nativeEntry{}, fmt.Errorf("%w: anchor identity changed", errNativeClaimConflict)
	}
	return pin, nil
}

// An already parked unknown can survive mount device renumbering. The original
// sending slot must be absent, and both retained names must still be links to
// the original inode and bytes. This check grants no authority to move, retire
// or resubmit either file. In-flight and terminal claim handling stays strict.
func nativeParkedAcrossDeviceChange(e nativeEntry) (bool, error) {
	src := filepath.Join(queueDir(), e.Name)
	if _, err := os.Lstat(src); !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	pin, err := nativeFingerprint(nativePinPath(e))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	oldDevice, oldInode, _ := strings.Cut(e.FileID, ":")
	newDevice, newInode, _ := strings.Cut(pin.FileID, ":")
	if _, err := strconv.ParseUint(oldDevice, 10, 64); err != nil {
		return false, nil
	}
	if oldDevice == newDevice || oldInode != newInode || pin.SHA256 != e.SHA256 {
		return false, nil
	}
	parked, err := nativeFingerprint(claimBase(src) + nativeUncertainSuffix)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return parked.FileID == pin.FileID && parked.ChangedAt == pin.ChangedAt &&
		parked.SHA256 == pin.SHA256, nil
}

func retireNativePin(r nativeIntent, e nativeEntry) error {
	pin, err := nativeAnchor(e)
	if errors.Is(err, errNativeClaimConflict) {
		if _, missing := os.Lstat(nativePinPath(e)); errors.Is(missing, os.ErrNotExist) {
			return nil
		} // already retired terminal reference
	}
	if err != nil {
		return err
	}
	if pin.SHA256 != e.SHA256 {
		// A completed refused publication transfers this inode to the queue and
		// permits that producer to replace its text before our extra link is retired.
		p, pe := os.Lstat(nativePinPath(e))
		q, qe := os.Lstat(finalClaimPath(r, filepath.Join(queueDir(), e.Name)))
		if r.Outcome != "refused" || pe != nil || qe != nil || !os.SameFile(p, q) {
			return fmt.Errorf("%w: anchor payload changed", errNativeClaimConflict)
		}
	}
	return ignoreNativeMissing(os.Remove(nativePinPath(e)))
}
