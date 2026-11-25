package hqnudge

import (
	"errors"
	"fmt"
	stdio "io"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// StorageUsage is a metadata-only sample. Claimed entries may be held for
// recovery and accepted records still require independent proof of delivery.
type StorageUsage struct {
	Queued, Claimed, Uncertain, AwaitingProof, Unclassified int
}

func InspectStorage() (StorageUsage, error) {
	var usage StorageUsage
	entries, err := wakeStorageNames(queueDir())
	if err != nil {
		return usage, err
	}
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
		case strings.HasSuffix(name, sendingSuffix), strings.HasSuffix(name, stuckSuffix):
			usage.Claimed++
		case isPendingWake(name):
			if _, ok := parseRetry(name); ok && attemptsOf(name) == 0 {
				usage.Queued++
			} else {
				usage.Unclassified++
			}
		default:
			usage.Unclassified++
		}
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
