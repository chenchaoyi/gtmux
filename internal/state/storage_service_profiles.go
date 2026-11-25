package state

import (
	"fmt"
	"path/filepath"
)

type StorageServiceProfileIdentity struct {
	Device uint64
	Inode  uint64
}

func StorageServiceRecoveryDir() string {
	return filepath.Join(Dir(), layoutRecovery, "launchd-maintenance")
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
