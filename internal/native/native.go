// Package native tracks agent sessions running OUTSIDE tmux — ones whose gtmux
// hooks fire (so we know they exist + their state) but which have no tmux pane to
// view or type into. Records are keyed by the agent's session_id (not a pane), so
// the state is independent of any terminal. The radar surfaces these as
// `source: "native"` rows; they are sense-only (no focus/jump, no send).
package native

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// StaleAfter is how long a native record may go without a hook update before it's
// treated as gone. A live idle session keeps within this via its hooks; the reap
// only drops sessions we've truly stopped hearing from. Generous so a genuinely
// idle-but-alive session isn't hidden. SessionEnd removes a record immediately.
const StaleAfter = 12 * time.Hour

// Record is one agent session sensed outside tmux.
type Record struct {
	Agent     string `json:"agent"`     // agent key (claude/codex/…)
	SessionID string `json:"sessionId"` // the agent's session id (the record key)
	Cwd       string `json:"cwd,omitempty"`
	State     string `json:"state"`     // working | waiting | idle
	UpdatedAt int64  `json:"updatedAt"` // unix seconds, last hook update
	// PID/Comm identify the agent PROCESS that fired the hook, so a "move to tmux"
	// can exit the original once the resumed session is up. Comm guards against PID
	// reuse (kill only if the pid is still that command). 0 = unknown (don't kill).
	PID  int    `json:"pid,omitempty"`
	Comm string `json:"comm,omitempty"`
}

// Dir is where per-session native records live.
func Dir() string { return filepath.Join(state.Dir(), "native") }

// fileFor maps a session id to its record file (base64url so any id is FS-safe).
func fileFor(sessionID string) string {
	return filepath.Join(Dir(), base64.RawURLEncoding.EncodeToString([]byte(sessionID))+".json")
}

// Save writes (overwriting) a native session record.
func Save(r Record) error {
	if r.SessionID == "" {
		return nil
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(fileFor(r.SessionID), b, 0o644)
}

// Load returns the record for a session id, ok=false if none/unreadable.
func Load(sessionID string) (Record, bool) {
	b, err := os.ReadFile(fileFor(sessionID))
	if err != nil {
		return Record{}, false
	}
	var r Record
	if json.Unmarshal(b, &r) != nil {
		return Record{}, false
	}
	return r, true
}

// Remove drops a native session record (e.g. on SessionEnd or after adoption).
func Remove(sessionID string) { _ = os.Remove(fileFor(sessionID)) }

// Live returns every native record that isn't stale, most-recently-updated first.
// A record older than StaleAfter is deleted as a side effect (we've stopped
// hearing from it) so the store self-prunes.
func Live(now int64) []Record {
	entries, err := os.ReadDir(Dir())
	if err != nil {
		return nil
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(Dir(), e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) != nil {
			continue
		}
		if now-r.UpdatedAt > int64(StaleAfter/time.Second) {
			_ = os.Remove(p)
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}
