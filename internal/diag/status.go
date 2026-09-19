package diag

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Status answers "what is true right now" for one component: one small JSON file,
// overwritten. A surface reads status to learn a component's state and never parses a
// log for it. The pairing window used to decide whether the tunnel was down by matching
// phrases in cloudflared's log, for either backend; a Direct user's verdict came from a
// tunnel they were not running.
type Status struct {
	Component  string         `json:"component"`
	Updated    time.Time      `json:"updated"`
	StaleAfter int            `json:"staleAfter"` // seconds
	PID        int            `json:"pid"`
	Version    string         `json:"version,omitempty"`
	State      string         `json:"state"`
	Since      time.Time      `json:"since"`
	Detail     map[string]any `json:"detail,omitempty"`
}

// version is stamped on every status; the entry point sets it once.
var version string

// SetVersion records the running gtmux version for status files.
func SetVersion(v string) { version = v }

// StatusPath is a component's status file.
func StatusPath(component string) string {
	return filepath.Join(state.StatusDir(), component+".json")
}

// Publish writes a component's status atomically. Since carries over while the state is
// unchanged, so it reads as "connected since 09:36", not "since the last heartbeat". A
// writer calls this at least every staleAfter/2, or readers will treat it as gone.
func Publish(component, st string, staleAfter time.Duration, detail map[string]any) {
	defer func() { _ = recover() }() // see write: publishing never takes the caller down
	t := now()
	since := t
	if prev, err := readStatus(component); err == nil && prev.State == st && !prev.Since.IsZero() {
		since = prev.Since
	}
	for k, v := range detail {
		if s, ok := v.(string); ok {
			detail[k] = Redact(s)
		}
	}
	b, err := json.MarshalIndent(Status{
		Component: component, Updated: t, StaleAfter: int(staleAfter.Seconds()),
		PID: os.Getpid(), Version: version, State: st, Since: since, Detail: detail,
	}, "", "  ")
	if err != nil {
		return
	}
	dir := state.StatusDir()
	if os.MkdirAll(dir, state.PrivateDir) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "."+component+"-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(append(b, '\n'))
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Chmod(tmp.Name(), state.PrivateFile) != nil ||
		os.Rename(tmp.Name(), StatusPath(component)) != nil {
		_ = os.Remove(tmp.Name())
	}
}

// ReadStatus returns a component's status and whether it is fresh. A status older than
// its staleAfter reads as not fresh: its writer has stopped, and the state it last wrote
// is no longer a fact.
func ReadStatus(component string) (st Status, fresh bool) {
	defer func() {
		if recover() != nil {
			st, fresh = Status{}, false
		}
	}()
	st, err := readStatus(component)
	if err != nil {
		return Status{}, false
	}
	fresh = st.StaleAfter > 0 && now().Sub(st.Updated) <= time.Duration(st.StaleAfter)*time.Second
	return st, fresh
}

func readStatus(component string) (Status, error) {
	var st Status
	b, err := os.ReadFile(StatusPath(component))
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(b, &st)
	return st, err
}
