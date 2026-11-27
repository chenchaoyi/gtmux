package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// InitializePurposeStorage publishes a complete new home in one no-replace
// rename. An existing root (including an unfamiliar or partially moved root)
// is never inferred to be fresh. Doctor inspection does not call this function.
func InitializePurposeStorage() error {
	root := Dir()
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	base := filepath.Dir(root)
	if err := MkdirAll(base, PrivateDir); err != nil {
		return err
	}
	fd, err := privateDirectory(base, false)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	stage, err := MkdirTemp(base, ".gtmux-storage-init-")
	if err != nil {
		return err
	}
	staged, err := os.Lstat(stage)
	if err != nil {
		return err
	}
	defer func() {
		if current, e := os.Lstat(stage); e == nil && os.SameFile(current, staged) {
			_ = os.RemoveAll(stage)
		}
	}()
	for _, d := range []string{layoutMarker, purposeMarker, "attachments/uploads", "diagnostics/logs", "sessions/terminal-input", "services/maintenance"} {
		if err := MkdirAll(filepath.Join(stage, d), PrivateDir); err != nil {
			return err
		}
	}
	token, err := purposeInputToken()
	if err != nil {
		return err
	}
	write := func(path string, data []byte) error {
		f, err := OpenFile(filepath.Join(stage, path), os.O_CREATE|os.O_WRONLY|os.O_EXCL, PrivateFile)
		if err != nil {
			return err
		}
		if _, err = f.Write(data); err == nil {
			err = f.Sync()
		}
		return errors.Join(err, f.Close())
	}
	data, _ := json.Marshal(inputGeneration{2, token})
	if err := write(purposeInputMarker, data); err != nil {
		return err
	}
	identity := func(rel string) (layoutEntry, error) {
		id, ok, err := layoutObject(filepath.Join(stage, rel))
		if err == nil && !ok {
			err = os.ErrNotExist
		}
		return id, err
	}
	queue, err := identity("sessions/terminal-input")
	if err != nil {
		return err
	}
	marker, err := identity(purposeInputMarker)
	if err != nil {
		return err
	}
	if err := write(purposeInputRecord, inputRecordBytes("active", token, queue, marker)); err != nil {
		return err
	}
	owner, err := identity(purposeMarker)
	if err != nil {
		return err
	}
	baseID, err := identity(layoutMarker)
	if err != nil {
		return err
	}
	j := &workflowJournal{BaseMarker: &baseID, Version: purposeVersion, Phase: purposeActive, PurposeToken: token, Marker: &owner}
	data, err = json.Marshal(j)
	if err != nil {
		return err
	}
	if err := write(layoutTransition, data); err != nil {
		return err
	}
	for _, key := range []string{"uploads", "logs"} {
		if err := os.Symlink(purposeRootStores[key], filepath.Join(stage, key)); err != nil {
			return err
		}
	}
	for _, d := range []string{"services/maintenance", "services", "attachments/uploads", "attachments", "diagnostics/logs", "diagnostics", "sessions/terminal-input", "sessions", layoutMarker, purposeMarker, "."} {
		if err := SyncPrivateDirectory(filepath.Join(stage, d)); err != nil {
			return err
		}
	}
	err = renameLayoutEntry(fd, filepath.Base(stage), fd, filepath.Base(root))
	if errors.Is(err, unix.EEXIST) {
		return nil
	}
	if err != nil {
		return err
	}
	return unix.Fsync(fd)
}
