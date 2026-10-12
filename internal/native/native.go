// Package native tracks agent sessions running OUTSIDE tmux — ones whose gtmux
// hooks fire (so we know they exist + their state) but which have no tmux pane to
// view or type into. Records are keyed by the agent's session_id (not a pane), so
// the state is independent of any terminal. The radar surfaces these as
// `source: "native"` rows; they are sense-only (no focus/jump, no send).
package native

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// StaleAfter is the grace for a record whose process cannot be checked (no pid on
// record, or ps could not answer): past it, with no hook update, the record goes. It
// does NOT apply to a process confirmed alive: an idle session sends no hooks, and the
// spec keeps an idle-but-alive one however long it idles. It used to apply to every
// record, so a session idle for twelve hours vanished while its process ran (%12,
// 2026-10-06). SessionEnd removes a record immediately.
const StaleAfter = 12 * time.Hour

// Record is one agent session sensed outside tmux.
type Record struct {
	HomeID         string `json:"home_id,omitempty"`         // opaque Codex home scope, empty on legacy/other agents
	CreationClient string `json:"creation_client,omitempty"` // creation metadata, never current ownership
	Revision       string `json:"revision,omitempty"`        // unique write generation, including same-second updates
	Event          string `json:"event,omitempty"`           // accepted hook; legacy records have no event
	Agent          string `json:"agent"`                     // agent key (claude/codex/…)
	SessionID      string `json:"sessionId"`                 // the agent's session id (the record key)
	Cwd            string `json:"cwd,omitempty"`
	State          string `json:"state"` // working | waiting | idle
	TurnID         string `json:"turnId,omitempty"`
	EndReason      string `json:"endReason,omitempty"` // interrupted | error; never a successful completion
	UpdatedAt      int64  `json:"updatedAt"`           // unix seconds, last hook update
	// Terminal is the app hosting the session ("Warp", "Ghostty", …), sensed from
	// the hook's own environment/ancestry at record time. "" when unrecognized.
	// Display-only (the radar row's `terminal` field); native jump stays deferred.
	Terminal string `json:"terminal,omitempty"`
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

// Save serializes and atomically publishes a fresh generation.
func Save(r Record) error {
	if r.SessionID == "" {
		return nil
	}
	return withStoreLock(func() error {
		r.Revision = diag.NewOpID()
		if !validHomeID(r.HomeID) {
			return errors.New("invalid native home identity")
		}
		path := recordPath(r)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		return atomicJSON(path, r)
	})
}

// Load returns the record for a session id, ok=false if none/unreadable.
func Load(sessionID string, homeIDs ...string) (Record, bool) {
	if len(homeIDs) > 0 {
		if !validHomeID(homeIDs[0]) {
			return Record{}, false
		}
		if r, ok := loadPath(recordPath(Record{SessionID: sessionID, HomeID: homeIDs[0]}), sessionID); ok {
			return r, true
		}
		return Record{}, false
	}
	var found Record
	count := 0
	for _, path := range recordCandidates(sessionID) {
		if r, ok := loadPath(path, sessionID); ok {
			found = r
			count++
		}
	}
	return found, count == 1
}

func loadPath(path, sessionID string) (Record, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Record{}, false
	}
	var r Record
	if json.Unmarshal(b, &r) != nil || r.SessionID != sessionID || !validHomeID(r.HomeID) || recordPath(r) != path {
		return Record{}, false
	}
	return r, true
}

// Remove drops a native session record (e.g. on SessionEnd or after adoption).
func Remove(sessionID string, homeIDs ...string) {
	if sessionID == "" {
		return
	}
	if err := withStoreLock(func() error {
		r, ok := Load(sessionID, homeIDs...)
		if !ok {
			return nil
		}
		return removeFile(recordPath(r))
	}); err != nil {
		diag.For("native").Warn("native.remove.failed", "could not remove native observation", "agent_session", sessionID, "error", err)
	}
}

// Live returns the native records whose agent process is alive, or cannot be checked
// and is still within StaleAfter, most-recently-updated first. It deletes (self-prunes)
// a record whose process is GONE — exited, killed, died in a reboot, or a pid since
// reused — the instant that is established, so a phantom "elsewhere" row does not
// linger; and a record it cannot check once StaleAfter passes. A process confirmed
// alive keeps its record however long it has been idle.
func Live(now int64) []Record {
	var out []Record
	for _, p := range observationFiles() {
		e, err := os.Stat(p)
		if err != nil {
			continue
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) != nil || r.SessionID == "" || !validHomeID(r.HomeID) || recordPath(r) != p {
			diag.For("native").Warn("native.read.failed", "invalid native observation", "file", e.Name())
			continue
		}
		// A binding receipt is published before record removal. Finish an interrupted
		// retirement only when the exact old generation still exists.
		if proof, ok := BindingFor(r.SessionID, r.HomeID); ok && proof.Retired && proof.Observation == r {
			removeSnapshot(r)
			continue
		}
		switch processState(r.PID, r.Comm, r.UpdatedAt) {
		case processGone:
			removeSnapshot(r)
			continue
		case processUnknown:
			if now-r.UpdatedAt > int64(StaleAfter/time.Second) {
				removeSnapshot(r)
				continue
			}
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

type liveness int

const (
	processUnknown liveness = iota // cannot be checked: the StaleAfter grace applies
	processAlive                   // the process that wrote the record is running
	processGone                    // it is not: exited, or its pid now belongs to another
)

// startSlack is the tolerance on the start-time comparison: ps reports elapsed time in
// whole seconds, read at a slightly different moment from the clock. It makes the check a
// presumption, not a proof: a process with the recorded command name that took the pid
// within startSlack of the record's last update cannot be told apart from the writer.
const startSlack = 120

// processState reports what can be established about the process a record was written
// for. Its pid-reuse guards matter most right after a reboot, when the OS hands a dead
// session's pid to an unrelated process:
//
//   - pid <= 0 (none recorded, e.g. an old record) → unknown.
//   - kill(pid, 0) == ESRCH → no such process → gone.
//   - comm recorded, and ps reads another command at the pid → gone, whatever else ps
//     could or could not read.
//   - the pid's process started more than startSlack after the record's last update → it
//     is not the process that wrote it → gone. This is what makes "alive" safe to keep
//     without a deadline: a newer process that took the pid, even one with the same
//     command name, cannot have written an update from before it started.
//   - either reading missing (no command, no start time) → unknown: a transient ps
//     hiccup drops nothing, and missing evidence keeps nothing past the grace.
//   - otherwise → alive.
func processState(pid int, comm string, updatedAt int64) liveness {
	if pid <= 0 {
		return processUnknown
	}
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return processGone
	}
	c := procComm(pid)
	if comm != "" && c != "" && c != comm {
		return processGone
	}
	start, ok := procStart(pid)
	if ok && start > updatedAt+startSlack {
		return processGone
	}
	if c == "" || !ok {
		return processUnknown
	}
	return processAlive
}

// procStart is when a live pid's process started (unix seconds), from ps's elapsed
// time. A variable so a test can stand in for an unanswerable ps.
var procStart = func(pid int) (int64, bool) {
	out, err := exec.Command("ps", "-o", "etime=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, false
	}
	secs, ok := parseEtime(strings.TrimSpace(string(out)))
	if !ok {
		return 0, false
	}
	return time.Now().Unix() - secs, true
}

// parseEtime reads ps's elapsed time, [[dd-]hh:]mm:ss, as seconds.
func parseEtime(s string) (int64, bool) {
	var days int64
	if d, rest, found := strings.Cut(s, "-"); found {
		n, err := strconv.ParseInt(d, 10, 64)
		if err != nil {
			return 0, false
		}
		days, s = n, rest
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var secs int64
	for _, p := range parts {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		secs = secs*60 + n
	}
	return days*86400 + secs, true
}

// procComm returns a live pid's short command name (macOS/Linux `ps`), base-named
// to match how the hook records Comm. "" when it can't be read.
func procComm(pid int) string {
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	c := strings.TrimSpace(string(out))
	if c == "" {
		return "" // ps answered with nothing: no command read (Base would make it ".")
	}
	return filepath.Base(c)
}
