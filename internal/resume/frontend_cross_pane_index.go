package resume

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Only the authenticated heartbeat can use this path. The caller holds the
// publication lease and restores the index only after preserving the original.
func preserveCrossPaneIndex(ctx context.Context, previous, next CodexFrontend, index Record) ([]byte, error) {
	conflict := &FrontendBindingError{Code: "frontend-index-conflict"}
	if index.Agent != "codex" || index.SessionID == "" || index.SessionID == previous.SessionID ||
		index.HomeID != previous.HomeID || index.Frontend != "" || index.TerminalClient != "" {
		return nil, conflict
	}
	own, ok := native.BindingFor(previous.SessionID, previous.HomeID)
	if !ok || own.Kind != "frontend" || own.ID != previous.Instance || own.Pane != previous.Pane ||
		own.Loc != previous.Loc || own.PanePID != previous.PanePID || own.PID != previous.PID ||
		own.BackendPID != previous.BackendPID || own.BackendStart != previous.BackendStart {
		return nil, conflict
	}
	foreign, ok := native.BindingFor(index.SessionID, index.HomeID)
	if !ok || foreign.Kind != "frontend" || foreign.BackendPID <= 1 || foreign.BackendStart == "" ||
		foreign.ID == own.ID || foreign.Cwd != index.Cwd || foreign.Pane == own.Pane || foreign.Loc == own.Loc ||
		foreign.PanePID == own.PanePID || foreign.PID == own.PID {
		return nil, conflict
	}

	if !producerHasCommittedIndexOwner(previous, next) {
		return nil, conflict
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := snapshotResumeIndex(next.Loc, index)
	if err != nil {
		return nil, err
	}
	unavailable := crossPaneIndexUnavailable()
	// Recommit equal visible bytes too: a previous atomic write can report a
	// failed directory sync after rename. Sync the parent of the archive
	// directory as well, since the first write may have just created it.
	path := crossPaneIndexHistoryPath(next.Loc, raw)
	archived, err := readCrossPaneIndex(path)
	if err == nil && !bytes.Equal(archived, raw) || err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, unavailable
	}
	if writeCrossPaneArchive(path, raw) != nil || syncCrossPaneArchiveParent(filepath.Dir(filepath.Dir(path))) != nil {
		return nil, unavailable
	}
	return raw, nil
}

var writeCrossPaneArchive = state.WritePrivateFile
var syncCrossPaneArchiveParent = state.SyncPrivateDirectory

func crossPaneIndexUnavailable() *FrontendBindingError {
	return &FrontendBindingError{Code: "frontend-index-preservation-unavailable", Retryable: true}
}

// All upgraded writers honor the same publication lease. Recheck the exact
// bytes immediately before publishing to catch observed old-binary writes too.
func snapshotResumeIndex(loc string, index Record) ([]byte, error) {
	raw, err := readCrossPaneIndex(fileFor(loc))
	var current Record
	if err != nil || json.Unmarshal(raw, &current) != nil || current != index {
		return nil, crossPaneIndexUnavailable()
	}
	return raw, nil
}

func verifyResumeIndexBytes(loc string, raw []byte) error {
	current, err := readCrossPaneIndex(fileFor(loc))
	if err != nil {
		return crossPaneIndexUnavailable()
	}
	if !bytes.Equal(current, raw) {
		return &FrontendBindingError{Code: "frontend-index-changed", Retryable: true}
	}
	return nil
}

func crossPaneIndexHistoryPath(loc string, raw []byte) string {
	return filepath.Join(Dir(), "codex-index-conflicts", fmt.Sprintf("%x-%x.json", sha256.Sum256([]byte(loc)), sha256.Sum256(raw)))
}

func readCrossPaneIndex(path string) ([]byte, error) {
	file, err := state.OpenPrivateRead(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("resume index is not private")
	}
	const limit = 8192
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if len(raw) > limit {
		return nil, errors.New("resume index exceeds recovery limit")
	}
	return raw, err
}
