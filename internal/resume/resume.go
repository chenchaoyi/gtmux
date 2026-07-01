// Package resume persists "which agent conversation was running in which pane"
// so `gtmux restore` can relaunch the conversation (e.g. `claude --resume <id>`)
// after a reboot — not just bring back the empty tmux pane.
//
// Records are keyed by an ABSTRACT stable location id, a plain string. The tmux
// host computes it as "session:window.pane" (the same coordinates tmux-resurrect
// restores by, so they survive a reboot). A future non-tmux host (e.g. cmux) can
// key by "workspace:surface" instead — the schema deliberately does not encode
// any tmux structure, so widening hosts needs no change here.
package resume

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Record is one resumable agent session captured from a hook.
type Record struct {
	Agent     string `json:"agent"`     // the agent key (claude/codex/cursor/…)
	SessionID string `json:"sessionId"` // the agent's resumable conversation id
	Cwd       string `json:"cwd,omitempty"`
	UpdatedAt int64  `json:"updatedAt"` // unix seconds, last time we saw this pane
}

// Dir is where per-location resume records live.
func Dir() string { return filepath.Join(state.Dir(), "resume") }

// fileFor maps a location key to its record file. The key is base64url-encoded so
// arbitrary session names (spaces, slashes, colons) are always filesystem-safe.
func fileFor(key string) string {
	return filepath.Join(Dir(), base64.RawURLEncoding.EncodeToString([]byte(key))+".json")
}

// Save writes (overwriting) the record for a location key.
func Save(key string, r Record) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(fileFor(key), b, 0o644)
}

// Load returns the record for a location key, ok=false if none/unreadable.
func Load(key string) (Record, bool) {
	b, err := os.ReadFile(fileFor(key))
	if err != nil {
		return Record{}, false
	}
	var r Record
	if json.Unmarshal(b, &r) != nil {
		return Record{}, false
	}
	return r, true
}

// Remove drops the record for a location key (best-effort).
func Remove(key string) { _ = os.Remove(fileFor(key)) }

// All returns every saved record, most-recent first. Restore uses it as a cwd
// fallback: when a restored pane's exact locator key didn't match (a session was
// renamed / windows reindexed), it can still find the conversation by working dir.
func All() []Record {
	entries, err := os.ReadDir(Dir())
	if err != nil {
		return nil
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(Dir(), e.Name()))
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}
