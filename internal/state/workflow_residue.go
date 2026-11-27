package state

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// RecoveryScaffold identifies empty service parking directories after a verified
// migration. Unknown children (including a recovery plan or parked job) protect
// the entire scaffold. Never enumerate or delete the contents recursively.
type RecoveryScaffold struct {
	root, services layoutEntry
}

func InspectRecoveryScaffold() (*RecoveryScaffold, error) {
	j, err := readWorkflowJournal()
	if err != nil || j == nil || (j.Phase != workflowGrouped && j.Phase != purposeActive) {
		return nil, err
	}
	if err := CheckStorageLayout(); err != nil {
		return nil, err
	}
	base := filepath.Join(Dir(), layoutRecovery)
	id, found, err := workflowPathObject(base)
	if err != nil || !found {
		return nil, err
	}
	if err := checkLayoutDir(base); err != nil {
		return nil, err
	}
	names, err := boundedWorkflowDirectory(base)
	if err != nil {
		return nil, err
	}
	plan := &RecoveryScaffold{root: id}
	if len(names) == 0 {
		return plan, nil
	}
	held := StorageServiceRecoveryDir()
	if len(names) != 1 || names[0].Name() != filepath.Base(held) {
		return nil, nil
	}
	if err := checkLayoutDir(held); err != nil {
		return nil, err
	}
	id, found, err = workflowPathObject(held)
	if err != nil || !found {
		return nil, err
	}
	names, err = boundedWorkflowDirectory(held)
	if err != nil || len(names) != 0 {
		return nil, err
	}
	plan.services = id
	return plan, nil
}

// Prune verifies both directory identities after consent. The caller holds the
// maintenance lease to serialize with service parking and restoration.
func (plan RecoveryScaffold) Prune() error {
	current, err := InspectRecoveryScaffold()
	if err != nil {
		return err
	}
	if current == nil || *current != plan {
		return errors.New("storage recovery directory changed during review")
	}
	if plan.services.Inode != 0 {
		if err := removeRecoveryDirectory(StorageServiceRecoveryDir(), plan.services); err != nil {
			return err
		}
	}
	return removeRecoveryDirectory(filepath.Join(Dir(), layoutRecovery), plan.root)
}

func removeRecoveryDirectory(path string, expected layoutEntry) error {
	parent, err := privateDirectory(filepath.Dir(path), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	id, err := layoutDirectoryAt(parent, filepath.Base(path))
	if err != nil {
		return err
	}
	if id != expected {
		return errors.New("storage recovery directory identity changed")
	}
	if err := unix.Unlinkat(parent, filepath.Base(path), unix.AT_REMOVEDIR); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return unix.Fsync(parent)
}
