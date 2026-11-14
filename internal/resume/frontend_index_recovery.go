package resume

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A live publisher can outlast a failed storage interval. This exceptional
// path renews only its exact in-memory committed owner after reobserving the
// whole terminal incarnation. It is called under the publication lease and
// the connection's existing epoch/lifetime permission. Readers never use it.
func producerCanRenewMissingIndex(previous, next CodexFrontend, saved CodexFrontend) bool {
	if previous.BridgePID != os.Getpid() || saved != previous || previous.Loc != next.Loc ||
		time.Now().UnixMilli()-saved.UpdatedAt <= 5000 {
		return false
	}
	if _, err := os.Lstat(fileFor(next.Loc)); !errors.Is(err, os.ErrNotExist) {
		return false // malformed/unreadable or concurrently occupied is not missing
	}
	checked := saved
	checked.UpdatedAt = next.UpdatedAt
	if producerFrontendProof(checked) != nil {
		return false
	}
	dir, err := os.Open(filepath.Join(Dir(), "codex-frontends"))
	if err != nil {
		return false
	}
	entries, err := dir.ReadDir(129)
	_ = dir.Close()
	if err != nil && err != io.EOF || len(entries) > 128 {
		return false
	}
	now := time.Now().UnixMilli()
	ownerStillPresent := false
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		other, ok := readFrontend(strings.TrimSuffix(entry.Name(), ".json"))
		if !ok {
			return false
		}
		if other.Instance == previous.Instance {
			if other != saved {
				return false
			}
			ownerStillPresent = true
			continue
		}
		if other.Loc != next.Loc {
			continue
		}
		age := now - other.UpdatedAt
		if age <= 5000 || other.StartedAt > previous.StartedAt ||
			other.BridgePID == previous.BridgePID && other.BridgeStart == previous.BridgeStart && other.Sequence > previous.Sequence {
			return false
		}
	}
	return ownerStillPresent
}
