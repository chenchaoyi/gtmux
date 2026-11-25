package state

import (
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type StorageServiceProfileIdentity struct {
	Device uint64
	Inode  uint64
}

func StorageServiceRecoveryDir() string {
	return filepath.Join(Dir(), layoutRecovery, "launchd-maintenance")
}

// PrepareWorkflowServiceFence drains old CLI admission before the first profile
// can be parked. The existing journal path also fences already installed builds
// after a process exit or a restart.
func PrepareWorkflowServiceFence() error {
	lease, err := acquireLayoutLease(unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer lease.Close()
	journal, err := readWorkflowJournal()
	if err != nil {
		return err
	}
	if journal != nil {
		switch {
		case journal.Phase == workflowServicePrep:
			if err := checkWorkflowServicePreparation(); err != nil {
				return err
			}
		case journal.Phase == workflowServiceMove:
			if err := verifyWorkflowMoves(journal, false); err != nil {
				return err
			}
		default:
			return errors.New("another storage change already owns the journal")
		}
		return SyncPrivateDirectory(Dir())
	}
	if err := CheckStorageLayout(); err != nil {
		return err
	}
	if err := checkWorkflowServicePreparation(); err != nil {
		return err
	}
	return publishWorkflowJournal(&workflowJournal{Version: workflowVersion, Phase: workflowServicePrep}, nil)
}

// ReleaseWorkflowServiceFence makes the verified layout available to ordinary
// commands only once no service profile remains held outside LaunchAgents.
func ReleaseWorkflowServiceFence() error {
	lease, err := acquireLayoutLease(unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer lease.Close()
	journal, err := readWorkflowJournal()
	if err != nil {
		return err
	}
	if err := verifyStorageBeforeReturningServices(journal); err != nil {
		return err
	}
	for _, label := range []string{"com.gtmux.serve", "com.gtmux.tunnel", "com.gtmux.selftunnel"} {
		if _, parked, _, err := FindStorageServiceProfile(label); err != nil || parked {
			return fmt.Errorf("launchd profile %s has not been returned", label)
		}
	}
	if journal == nil || journal.Phase == workflowGrouped {
		return SyncPrivateDirectory(Dir())
	}
	if journal.Phase == workflowServiceReady {
		active := &workflowJournal{Version: workflowVersion, Phase: workflowGrouped, Marker: journal.Marker}
		return publishWorkflowJournal(active, journal)
	}
	parent, err := privateDirectory(Dir(), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	if err := verifyWorkflowJournal(journal); err != nil {
		return err
	}
	if current, err := layoutRegularAt(parent, layoutTransition); err != nil || current != journal.proof.file {
		return errors.New("prepared service journal changed before retirement")
	}
	if err := unix.Unlinkat(parent, layoutTransition, 0); err != nil {
		return err
	}
	return unix.Fsync(parent)
}

// FindStorageServiceProfile reads the two possible names of a managed profile.
// The recovery name sits outside the LaunchAgents search path.
func FindStorageServiceProfile(label string) (path string, parked bool, identity StorageServiceProfileIdentity, err error) {
	live, held, err := storageServiceProfilePaths(label)
	if err != nil {
		return "", false, identity, err
	}
	a, here, err := layoutObject(live)
	if err != nil {
		return "", false, identity, err
	}
	b, there, err := layoutObject(held)
	if err != nil {
		return "", false, identity, err
	}
	switch {
	case here && there || here && a.Mode != 0 || there && b.Mode != 0:
		return "", false, identity, fmt.Errorf("launchd profile %s has conflicting or invalid paths", label)
	case here:
		return live, false, StorageServiceProfileIdentity{a.Dev, a.Inode}, nil
	case there:
		return held, true, StorageServiceProfileIdentity{b.Dev, b.Inode}, nil
	default:
		return "", false, identity, nil
	}
}

func storageServiceProfilePaths(label string) (string, string, error) {
	switch label {
	case "com.gtmux.serve", "com.gtmux.tunnel", "com.gtmux.selftunnel":
		name := label + ".plist"
		return filepath.Join(Home(), "Library", "LaunchAgents", name), filepath.Join(StorageServiceRecoveryDir(), name), nil
	default:
		return "", "", fmt.Errorf("unrecognized storage maintenance service %q", label)
	}
}

// MoveStorageServiceProfile preserves the single original inode and never
// replaces another file. Both directory entries are synced even on retry.
func MoveStorageServiceProfile(label string, id StorageServiceProfileIdentity, park bool) error {
	live, parked, err := storageServiceProfilePaths(label)
	if err != nil {
		return err
	}
	if !park {
		live, parked = parked, live
	}
	return advanceLayoutEntry(layoutEntry{Dev: id.Device, Inode: id.Inode}, live, parked)
}
