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
func preserveCrossPaneIndex(ctx context.Context, previous, next CodexFrontend, index Record) error {
	conflict := &FrontendBindingError{Code: "frontend-index-conflict"}
	if index.Agent != "codex" || index.SessionID == "" || index.SessionID == previous.SessionID ||
		index.HomeID != previous.HomeID || index.Frontend != "" || index.TerminalClient != "" {
		return conflict
	}
	own, ok := native.BindingFor(previous.SessionID, previous.HomeID)
	if !ok || own.Kind != "frontend" || own.ID != previous.Instance || own.Pane != previous.Pane ||
		own.Loc != previous.Loc || own.PanePID != previous.PanePID || own.PID != previous.PID ||
		own.BackendPID != previous.BackendPID || own.BackendStart != previous.BackendStart {
		return conflict
	}
	foreign, ok := native.BindingFor(index.SessionID, index.HomeID)
	if !ok || foreign.Kind != "frontend" || foreign.BackendPID <= 1 || foreign.BackendStart == "" ||
		foreign.Cwd != index.Cwd || foreign.Pane == own.Pane || foreign.Loc == own.Loc ||
		foreign.PanePID == own.PanePID || foreign.PID == own.PID {
		return conflict
	}

	saved, ok := readFrontend(previous.Instance)
	last := saved
	last.UpdatedAt = previous.UpdatedAt
	// A failed index write may have refreshed this same receipt. Accept only
	// an exact match apart from the newer timestamp.
	if !ok || last != previous || saved.UpdatedAt < previous.UpdatedAt || saved.UpdatedAt > next.UpdatedAt ||
		!producerHasUniqueIndexProof(previous, next, saved) {
		return conflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	unavailable := &FrontendBindingError{Code: "frontend-index-preservation-unavailable", Retryable: true}
	raw, err := readCrossPaneIndex(fileFor(next.Loc))
	var current Record
	if err != nil || json.Unmarshal(raw, &current) != nil || current != index {
		return unavailable
	}
	// Content addressing keeps retries idempotent and distinct displaced
	// versions separate. Normal resume and input readers never use this path.
	path := crossPaneIndexHistoryPath(next.Loc, raw)
	archived, err := readCrossPaneIndex(path)
	switch {
	case err == nil && bytes.Equal(archived, raw):
		return nil
	case errors.Is(err, os.ErrNotExist):
		if state.WritePrivateFile(path, raw) == nil {
			return nil
		}
	}
	return unavailable
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
		return nil, errors.New("Codex resume index is not private")
	}
	const limit = 8192
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if len(raw) > limit {
		return nil, errors.New("Codex resume index exceeds recovery limit")
	}
	return raw, err
}
