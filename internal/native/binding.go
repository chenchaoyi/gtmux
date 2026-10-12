package native

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// Binding is a receipt from the exact submitted-task binding path, not a guess
// from directory or inherited TMUX_PANE. It records no prompt or environment.
type Binding struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	Pane        string `json:"pane"`
	Loc         string `json:"loc"`
	Cwd         string `json:"cwd"`
	PID         int    `json:"pid"`
	PanePID     int    `json:"pane_pid"`
	PreparedAt  int64  `json:"prepared_at"`
	BoundAt     int64  `json:"bound_at"`
	Retired     bool   `json:"retired"`
	Observation Record `json:"observation"`
}

const bindingLimit = 256

func bindingsDir() string { return filepath.Join(Dir(), "bindings") }
func bindingPath(id string) string {
	return filepath.Join(bindingsDir(), base64.RawURLEncoding.EncodeToString([]byte(id))+".json")
}

func BindingFor(id string) (Binding, bool) {
	v, err := readBinding(id)
	if err != nil {
		diag.For("native").Warn("native.binding.read.failed", "could not read binding receipt", "agent_session", id, "error", err)
		return Binding{}, false
	}
	return v, v.ID != ""
}

func readBinding(id string) (Binding, error) {
	b, err := os.ReadFile(bindingPath(id))
	if os.IsNotExist(err) {
		return Binding{}, nil
	}
	if err != nil {
		return Binding{}, err
	}
	var v Binding
	if err := json.Unmarshal(b, &v); err != nil {
		return Binding{}, err
	}
	if v.SessionID != id || v.ID == "" || v.Pane == "" || v.Loc == "" || v.Cwd == "" || v.PID <= 0 || v.PanePID <= 0 || v.PreparedAt <= 0 || v.BoundAt < v.PreparedAt || v.Retired && v.Observation.SessionID != id {
		return Binding{}, errors.New("invalid binding receipt")
	}
	return v, nil
}

func provisional(r Record, b Binding) bool {
	// Without an explicit legacy start witness, only its minimal PID-less idle
	// startup shape is eligible. Real turns/processes/terminal attachments survive.
	return r.Agent == "codex" && r.SessionID == b.SessionID && r.Cwd == b.Cwd &&
		r.PID == 0 && r.Comm == "" && r.Terminal == "" && r.State == "idle" &&
		r.TurnID == "" && r.EndReason == "" && (r.Event == "" || r.Event == "SessionStart") &&
		r.UpdatedAt > 0 && r.UpdatedAt <= b.BoundAt && (r.Event == "SessionStart" || r.UpdatedAt <= b.PreparedAt) && r.UpdatedAt >= b.PreparedAt-600
}

// ConfirmBinding publishes evidence before deleting an exact provisional snapshot.
// Save and prune share the lock; a newer generation wins even in the same second.
// On failure the binding intent must remain retryable in the caller.
func ConfirmBinding(b Binding, expected Record) error {
	if b.ID == "" || b.SessionID == "" || b.Pane == "" || b.Loc == "" || b.Cwd == "" ||
		b.PID <= 0 || b.PanePID <= 0 || b.PreparedAt <= 0 || b.BoundAt < b.PreparedAt {
		return errors.New("incomplete verified binding receipt")
	}
	return withStoreLock(func() error {
		if err := os.MkdirAll(bindingsDir(), 0700); err != nil {
			return err
		}
		// A later binding cannot be overwritten by a delayed consumer.
		previous, err := readBinding(b.SessionID)
		if err != nil {
			return err
		}
		if previous.ID != "" && (previous.BoundAt > b.BoundAt || previous.BoundAt == b.BoundAt && previous.ID != b.ID) {
			return errors.New("binding receipt has a newer owner")
		}
		current, exists := Load(b.SessionID)
		b.Observation = expected
		b.Retired = exists && current == expected && provisional(expected, b)
		if old := previous; old.ID == b.ID && old.Retired {
			// A retry after publication/removal failure must retain its original CAS.
			b.Observation, b.Retired = old.Observation, old.Retired
		}
		if err := atomicJSON(bindingPath(b.SessionID), b); err != nil {
			return err
		}
		if b.Retired && exists && current == b.Observation {
			if err := removeFile(fileFor(b.SessionID)); err != nil {
				return err
			}
			diag.For("native").Info("native.binding.retired", "retired provisional native observation after exact tmux binding", "agent_session", b.SessionID, "pane", b.Pane, "loc", b.Loc, "binding_id", b.ID, "revision", b.Observation.Revision)
		}
		pruneBindings()
		return nil
	})
}

// Bound historical receipts; those with existing observations survive for recovery.
func pruneBindings() {
	entries, err := os.ReadDir(bindingsDir())
	if err != nil {
		return
	}
	type oldFile struct {
		path string
		time int64
	}
	var completed []oldFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id, err := base64.RawURLEncoding.DecodeString(entry.Name()[:len(entry.Name())-5])
		if err != nil {
			continue
		}
		if _, ok := Load(string(id)); ok {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			completed = append(completed, oldFile{filepath.Join(bindingsDir(), entry.Name()), info.ModTime().UnixNano()})
		}
	}
	sort.Slice(completed, func(i, j int) bool { return completed[i].time < completed[j].time })
	for len(completed) > bindingLimit {
		if err := removeFile(completed[0].path); err != nil {
			diag.For("native").Warn("native.binding.prune.failed", "could not expire binding receipt", "error", err)
		}
		completed = completed[1:]
	}
}
