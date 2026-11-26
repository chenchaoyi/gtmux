package hqnudge

import (
	"errors"
	"fmt"
	stdio "io"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// StorageUsage counts directory entries by stage and distinct-file conflicts
// by wake slot. A claimed entry may be held for recovery, and an accepted
// record still requires independent proof of delivery.
type StorageUsage struct {
	Queued, Claimed, Refused, Uncertain, AwaitingProof, Unclassified int
	ConflictingSlots                                                 int
}

func InspectStorage() (StorageUsage, error) {
	var usage StorageUsage
	entries, err := wakeStorageNames(queueDir())
	if err != nil {
		return usage, err
	}
	var claims []os.DirEntry
	parked := make(map[string]os.DirEntry)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() {
			usage.Unclassified++
			continue
		}
		switch {
		case name == "fail-count" || name == "enter-repair":
			// These record recovery bookkeeping rather than delivery items.
		case strings.HasSuffix(name, nativeUncertainSuffix), strings.HasSuffix(name, legacyUncertainSuffix):
			usage.Uncertain++
			parked[name] = entry
		case strings.HasSuffix(name, wakeRefusedSuffix):
			base := strings.TrimSuffix(name, wakeRefusedSuffix)
			if _, valid := parseRetry(base); !isPendingWake(base) || !valid {
				usage.Unclassified++
				continue
			}
			usage.Refused++
			parked[name] = entry
		case strings.HasSuffix(name, sendingSuffix), strings.HasSuffix(name, stuckSuffix):
			usage.Claimed++
			claims = append(claims, entry)
		case isPendingWake(name):
			_, valid := parseRetry(name)
			attempts := attemptsOf(name)
			switch {
			case !valid || attempts < 0:
				usage.Unclassified++
			case attempts > 0:
				// Older writers encode an unconfirmed paste in the queue name.
				usage.Uncertain++
			default:
				usage.Queued++
			}
		default:
			usage.Unclassified++
		}
	}
	usage.ConflictingSlots, err = wakeStorageConflicts(claims, parked)
	if err != nil {
		return usage, err
	}
	entries, err = wakeStorageNames(acceptedWakeDir())
	if err != nil {
		return usage, err
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".json") {
			usage.AwaitingProof++
		} else {
			usage.Unclassified++
		}
	}
	return usage, nil
}

func wakeStorageConflicts(claims []os.DirEntry, parked map[string]os.DirEntry) (int, error) {
	conflicts := make(map[string]bool)
	for _, claim := range claims {
		slot := claimBase(claim.Name())
		if conflicts[slot] {
			continue
		}
		for _, suffix := range []string{nativeUncertainSuffix, legacyUncertainSuffix, wakeRefusedSuffix} {
			retained := parked[slot+suffix]
			if retained == nil {
				continue
			}
			different, err := distinctWakeFiles(claim, retained)
			if err != nil {
				return 0, err
			}
			if different {
				conflicts[slot] = true
				break
			}
		}
	}
	return len(conflicts), nil
}

func distinctWakeFiles(claim, retained os.DirEntry) (bool, error) {
	a, err := claim.Info()
	if err != nil {
		return false, ignoreNativeMissing(err)
	}
	b, err := retained.Info()
	if err != nil {
		return false, ignoreNativeMissing(err)
	}
	// Publication links the retained name before removing the claim. The
	// two names can temporarily point to the same file during a normal move.
	return a.Mode().IsRegular() && b.Mode().IsRegular() && !os.SameFile(a, b), nil
}

func wakeStorageNames(path string) ([]os.DirEntry, error) {
	if err := state.CheckPrivateDirectory(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	const limit = 8192
	entries, err := file.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, stdio.EOF) {
		return nil, err
	}
	if len(entries) > limit {
		return nil, fmt.Errorf("wake inventory exceeds its scan budget: %s", path)
	}
	return entries, nil
}
