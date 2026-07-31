package servermode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// StaleAfter is how long a heartbeat may go unrefreshed before the state is
// considered abandoned. It is a LIVENESS signal, never an expiry: server mode has
// no time limit, so a fresh heartbeat means "gtmux is still here to keep serving",
// not "the lease is still valid".
const StaleAfter = 120 * time.Second

// Record is gtmux's own note that IT disabled sleep — the ownership stamp. Its
// existence is what separates "we did this" (ours to clean up) from "someone else
// did this" (report only, never revert).
type Record struct {
	Tier        string `json:"tier"`
	Since       int64  `json:"since"`
	HeartbeatAt int64  `json:"heartbeat_at"`
}

// StateDir holds gtmux's own view. The directory keeps the "server-mode" name even
// though the command is `gtmux awake` — the FEATURE is still server mode, and
// renaming a state path would need a migration for no user-visible gain.
//
// StateDir holds gtmux's own view. The guard's record lives elsewhere (see
// GuardExitPath) because the guard runs as root with no user session.
func StateDir() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "gtmux", "server-mode")
}

func StatePath() string { return filepath.Join(StateDir(), "state.json") }

// GuardExitPath is where the privileged guard records why it restored sleep.
//
// It is deliberately NOT under $HOME or a temp dir: the guard may run at boot with
// nobody logged in, and `boot-reconcile` exists precisely to explain something that
// happened before the machine came back — a record the OS clears on restart would
// explain nothing. (A temp dir cost this project one entire test run.)
func GuardExitPath() string {
	return "/Library/Application Support/gtmux/last-exit.json"
}

// LoadState reads the ownership stamp. A missing file is not an error — it is the
// normal state of a machine where server mode was never turned on.
func LoadState() (Record, bool) {
	b, err := os.ReadFile(StatePath())
	if err != nil {
		return Record{}, false
	}
	var r Record
	if json.Unmarshal(b, &r) != nil {
		return Record{}, false
	}
	return r, true
}

// Stale reports whether the heartbeat has gone unrefreshed past StaleAfter. A zero
// heartbeat counts as stale: a record with no liveness at all cannot vouch for a
// running gtmux.
func (r Record) Stale(now time.Time) bool {
	if r.HeartbeatAt == 0 {
		return true
	}
	return now.Sub(time.Unix(r.HeartbeatAt, 0)) > StaleAfter
}

// LoadLastExit merges the guard's record. Best effort by design: it is written by
// root for display, and must never be treated as authority over current state.
func LoadLastExit() (Exit, bool) {
	b, err := os.ReadFile(GuardExitPath())
	if err != nil {
		return Exit{}, false
	}
	var e Exit
	if json.Unmarshal(b, &e) != nil || e.Reason == "" {
		return Exit{}, false
	}
	return e, true
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
