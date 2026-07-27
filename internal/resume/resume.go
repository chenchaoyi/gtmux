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

// Located pairs a record with the location key it was saved under (decoded from the
// filename). Restore's cwd fallback needs the ORIGINAL locator — specifically the
// window.pane layout position inside it — to tell a pane that truly hosted an agent
// (its session was merely renamed, position preserved) from a bare shell pane that
// only shares a project directory. Without that evidence the fallback injected a
// historical conversation into every project-dir shell pane after a restore (the
// "multiple cc sessions after reboot" bug).
type Located struct {
	Loc string // the location key this record was saved under (e.g. "session:window.pane")
	Record
}

// All returns every saved record, most-recent first (nil on an empty store).
func All() []Record {
	loc := AllLocated()
	if loc == nil {
		return nil
	}
	out := make([]Record, len(loc))
	for i := range loc {
		out[i] = loc[i].Record
	}
	return out
}

// AllLocated returns every saved record paired with its decoded location key,
// most-recent first. Restore uses it for the cwd fallback so it can require the
// record's original window.pane position to match the restored pane.
func AllLocated() []Located {
	entries, err := os.ReadDir(Dir())
	if err != nil {
		return nil
	}
	var out []Located
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(Dir(), e.Name()))
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) != nil {
			continue
		}
		key, err := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue // an unexpected filename — skip rather than guess its locator
		}
		out = append(out, Located{Loc: string(key), Record: r})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}
