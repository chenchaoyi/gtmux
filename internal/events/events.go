// Package events is the session-events layer (see openspec session-events): an
// append-only, size-ROTATED log of every agent lifecycle event, fed by the hook
// (the same source as the state markers / notify queue) so gtmux HQ and any
// consumer can SUBSCRIBE to all sessions' execution by tailing it — the
// terminal-native equivalent of the apps' SSE stream.
//
// Growth is bounded by rotation, never truncation-in-place (which would break a
// follower): when the active file passes the cap it is renamed to a single
// rotated generation and a fresh file starts. Ceiling ≈ (cap × 2).
package events

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Record is one logged lifecycle event (the stable additive contract).
type Record struct {
	Ts      int64  `json:"ts"`             // unix seconds
	Event   string `json:"event"`          // Stop | Waiting | Notification | UserPromptSubmit | …
	State   string `json:"state"`          // derived: working | waiting | idle | …
	Pane    string `json:"pane,omitempty"` // tmux pane id ("" for native)
	Loc     string `json:"loc,omitempty"`
	Session string `json:"session,omitempty"`
	Agent   string `json:"agent,omitempty"`
	Kind    string `json:"kind,omitempty"` // waiting kind: permission | plan | question
}

// Path is the active event log.
func Path() string { return filepath.Join(state.Dir(), "events.jsonl") }

// rotatedPath is the single retained older generation.
func rotatedPath() string { return filepath.Join(state.Dir(), "events.1.jsonl") }

// defaultCapBytes is the active-file size cap (config eventsCapMB, default 20).
const defaultCapMB = 20

// capBytes reads the configured cap (~/.config/gtmux/config.json eventsCapMB),
// defaulting to 20 MB. A non-positive value disables the log entirely (0 cap).
func capBytes() int64 {
	mb := defaultCapMB
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json"))
	if err == nil {
		var c struct {
			EventsCapMB *int `json:"eventsCapMB"`
		}
		if json.Unmarshal(b, &c) == nil && c.EventsCapMB != nil {
			mb = *c.EventsCapMB
		}
	}
	return int64(mb) << 20
}

// Append writes one record (best-effort — a telemetry log must never fail a
// hook). It rotates FIRST when the active file is at/over the cap, so the log
// can never single-point-explode.
func Append(r Record) {
	cap := capBytes()
	if cap <= 0 {
		return // disabled
	}
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		return
	}
	rotateIfNeeded(cap)
	line, err := json.Marshal(r)
	if err != nil {
		return
	}
	f, err := os.OpenFile(Path(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	// O_APPEND + a single small write is atomic across concurrent hooks (one line
	// never interleaves with another).
	_, _ = f.Write(append(line, '\n'))
	_ = f.Close()
}

// rotateIfNeeded renames the active file to the single rotated generation when it
// reaches the cap. A rename is cheap + atomic-ish; a concurrent appender simply
// opens the fresh file on its next Append.
func rotateIfNeeded(cap int64) {
	fi, err := os.Stat(Path())
	if err != nil || fi.Size() < cap {
		return
	}
	_ = os.Rename(Path(), rotatedPath()) // overwrites any prior generation
}

// Read returns records from the last `sinceSecs` seconds (0 = all retained),
// oldest-first, spanning the rotated generation so a recent window isn't cut at
// a rotation boundary. now is the reference time in unix seconds.
func Read(sinceSecs, now int64) []Record {
	var out []Record
	cutoff := int64(0)
	if sinceSecs > 0 {
		cutoff = now - sinceSecs
	}
	for _, p := range []string{rotatedPath(), Path()} { // oldest generation first
		out = append(out, readFile(p, cutoff)...)
	}
	return out
}

func readFile(path string, cutoff int64) []Record {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		if cutoff > 0 && r.Ts < cutoff {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Format renders a record as one compact human line (shared by the CLI printer
// and any consumer): "HH:MM:SS  state·kind  loc  agent  (event)".
func Format(r Record) string {
	var b strings.Builder
	b.WriteString(clock(r.Ts))
	b.WriteString("  ")
	st := r.State
	if r.Kind != "" {
		st += "·" + r.Kind
	}
	b.WriteString(pad(st, 16))
	loc := r.Loc
	if loc == "" {
		loc = r.Session
	}
	b.WriteString("  ")
	b.WriteString(pad(loc, 14))
	if r.Agent != "" {
		b.WriteString("  ")
		b.WriteString(r.Agent)
	}
	if r.Pane != "" {
		b.WriteString(" (" + r.Pane + ")")
	}
	return b.String()
}

// clock renders a record's unix ts as HH:MM:SS (local), "--:--:--" for 0.
func clock(ts int64) string {
	if ts <= 0 {
		return "--:--:--"
	}
	return time.Unix(ts, 0).Format("15:04:05")
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
