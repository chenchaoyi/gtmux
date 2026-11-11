package resume

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// ErrConversationUnassociated distinguishes missing proof from an empty log.
var ErrConversationUnassociated = errors.New("conversation association unavailable")

// ConversationRecord can recover a read view from independent current frontend
// evidence. It never repairs disk or changes LoadLive's input/restore authority.
func ConversationRecord(loc string) (Record, bool, error) {
	if r, ok := LoadLive(loc); ok && r.Agent != "" && r.SessionID != "" {
		if r.Agent != "codex" || r.HomeID == transcript.CodexHomeID() || r.HomeID == "" {
			return r, false, nil
		}
	}
	// A valid incumbent, even stale or foreign, is not silently replaced by a
	// different conversation. Only absent/unparseable primary indices recover.
	if _, exists := Load(loc); exists {
		return Record{}, false, ErrConversationUnassociated
	}
	f, ok := conversationFrontend(loc)
	if !ok {
		return Record{}, false, ErrConversationUnassociated
	}
	return frontendRecord(f), true, nil
}

func frontendRecord(f CodexFrontend) Record {
	return Record{Agent: "codex", HomeID: f.HomeID, SessionID: f.SessionID, Cwd: f.ThreadCwd, Frontend: f.Instance, UpdatedAt: f.UpdatedAt / 1000}
}

// Bounded exceptional path, not a global history scan. Unreadable receipts,
// ambiguous frontends and oversized stores cannot establish unique ownership.
func conversationFrontend(loc string) (CodexFrontend, bool) {
	dir, err := os.Open(filepath.Join(Dir(), "codex-frontends"))
	if err != nil {
		return CodexFrontend{}, false
	}
	entries, err := dir.ReadDir(129)
	_ = dir.Close()
	if err != nil && err != io.EOF || len(entries) > 128 {
		return CodexFrontend{}, false
	}
	var found CodexFrontend
	var observed []CodexFrontend
	now := time.Now().UnixMilli()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		f, ok := readFrontend(strings.TrimSuffix(entry.Name(), ".json"))
		if !ok {
			return CodexFrontend{}, false
		}
		if f.Loc != loc {
			continue
		}
		observed = append(observed, f)
		if age := now - f.UpdatedAt; age < -1000 || age > 5000 {
			continue
		}
		if found.Instance != "" {
			return CodexFrontend{}, false
		}
		found = f
	}
	if found.Instance == "" || !freshFrontend(found, true) {
		return CodexFrontend{}, false
	}
	for _, f := range observed {
		if f.Instance != found.Instance && (f.StartedAt > found.StartedAt || f.BridgePID == found.BridgePID && f.BridgeStart == found.BridgeStart && f.Sequence > found.Sequence) {
			return CodexFrontend{}, false
		}
	}
	return found, true
}
