// Package diag is gtmux's local log: one store on the Mac that every gtmux process
// writes to, holding what gtmux saw (diagnostics) and what it did (actions), in the
// spirit of the macOS unified log. See openspec change `diagnostics`.
//
// It exists because there was none. On 2026-09-19 a phone could not pair and nobody
// could say why: the phone kept no record, the menu bar kept none unless started by
// hand, and serve wrote nothing once it was running. Apart from seven acts the journal
// audits for HQ, nothing gtmux did to the Mac left a trace either.
//
// The store is ~/.local/share/gtmux/logs/, one JSON Lines file per local day. Every
// entry is appended in one write of at most MaxEntry bytes, which a local file system
// appends atomically, so the hook, serve and a command can all write at once. Logging
// never fails the operation that was logging: every error here is swallowed.
//
// The journal (internal/events) stays separate. HQ reads it; this is for a person
// debugging and for the record of what happened.
package diag

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Level orders entries by how much they matter.
type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

func (l Level) String() string {
	switch l {
	case Debug:
		return "debug"
	case Warn:
		return "warn"
	case Error:
		return "error"
	}
	return "info"
}

// ParseLevel reads a level name; an unknown name reads as Info.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return Debug
	case "warn", "warning":
		return Warn
	case "error":
		return Error
	}
	return Info
}

// The two kinds of entry.
const (
	KindDiag = "diag" // what gtmux saw
	KindAct  = "act"  // what gtmux did
)

// Action outcomes.
const (
	OK      = "ok"
	Refused = "refused"
	Failed  = "failed"
)

// MaxEntry bounds one entry: a single write this size or smaller appends atomically.
const MaxEntry = 4096

// Entry is one line of the store. Action entries carry Actor, Target and Outcome.
type Entry struct {
	TS        string         `json:"ts"`
	Level     string         `json:"level"`
	Component string         `json:"component"`
	Kind      string         `json:"kind"`
	Event     string         `json:"event"`
	Actor     string         `json:"actor,omitempty"`
	Target    string         `json:"target,omitempty"`
	Outcome   string         `json:"outcome,omitempty"`
	Msg       string         `json:"msg,omitempty"`
	Attrs     map[string]any `json:"attrs,omitempty"`
	Truncated bool           `json:"truncated,omitempty"`
}

// Time parses the entry's timestamp.
func (e Entry) Time() time.Time {
	t, _ := time.Parse(tsLayout, e.TS)
	return t
}

const tsLayout = "2006-01-02T15:04:05.000Z07:00"

// now is swappable so tests can move the clock across days.
var now = time.Now

// Logger writes entries for one component.
type Logger struct{ component string }

// For returns the logger for a component: serve, tunnel, hook, cli, hq, restore,
// hygiene, menubar.
func For(component string) *Logger { return &Logger{component: component} }

// Debug, Info, Warn and Error record what the component saw. kv is a flat list of
// key, value pairs; values should be scalars.
func (l *Logger) Debug(event, msg string, kv ...any) { l.diag(Debug, event, msg, kv) }
func (l *Logger) Info(event, msg string, kv ...any)  { l.diag(Info, event, msg, kv) }
func (l *Logger) Warn(event, msg string, kv ...any)  { l.diag(Warn, event, msg, kv) }
func (l *Logger) Error(event, msg string, kv ...any) { l.diag(Error, event, msg, kv) }

func (l *Logger) diag(lv Level, event, msg string, kv []any) {
	if lv == Debug && !debugOn(l.component) {
		return
	}
	write(Entry{Level: lv.String(), Component: l.component, Kind: KindDiag,
		Event: event, Msg: msg, Attrs: attrs(kv)})
}

// Act records something gtmux did: who started it, what it acted on, how it ended.
// A refused action is a warning and a failed one an error, so `gtmux logs --level
// warn` shows every action that did not happen.
func (l *Logger) Act(event, actor, target, outcome, msg string, kv ...any) {
	lv := Info
	switch outcome {
	case Refused:
		lv = Warn
	case Failed:
		lv = Error
	}
	write(Entry{Level: lv.String(), Component: l.component, Kind: KindAct, Event: event,
		Actor: actor, Target: target, Outcome: outcome, Msg: msg, Attrs: attrs(kv)})
}

func attrs(kv []any) map[string]any {
	if len(kv) < 2 {
		return nil
	}
	m := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		switch v := kv[i+1].(type) {
		case string, bool, int, int64, float64:
			m[k] = v
		case error:
			if v != nil {
				m[k] = v.Error()
			}
		case time.Duration:
			m[k] = v.Milliseconds()
		case nil:
		default:
			m[k] = jsonScalar(v)
		}
	}
	return m
}

func jsonScalar(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(b)
}

var writeMu sync.Mutex

// write stamps, redacts, bounds and appends one entry. It never returns an error: a log
// that cannot be written must not take the logging operation down with it.
func write(e Entry) {
	// Nothing in the logging path may take the caller down, including the guard in
	// state that panics when a test reaches for the real home: a test that never meant
	// to log must not fail because the code under it now does.
	defer func() { _ = recover() }()
	t := now()
	e.TS = t.Format(tsLayout)
	redactEntry(&e)
	line := encode(e)
	if line == nil {
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	dir := state.LogsDir()
	if err := os.MkdirAll(dir, state.PrivateDir); err != nil {
		return
	}
	day := t.Format("2006-01-02")
	maybeCleanupForNewDay(dir, day)
	path := segmentFor(dir, day, e)
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, state.PrivateFile)
	if err != nil {
		return
	}
	_, _ = f.Write(line)
	_ = f.Close()
}

// encode marshals an entry into one line of at most MaxEntry bytes, cutting the message
// and then the longest string attributes if it has to.
func encode(e Entry) []byte {
	b, err := json.Marshal(e)
	if err != nil {
		return nil
	}
	for len(b)+1 > MaxEntry {
		e.Truncated = true
		if !shrink(&e) {
			return nil
		}
		if b, err = json.Marshal(e); err != nil {
			return nil
		}
	}
	return append(b, '\n')
}

func shrink(e *Entry) bool {
	longest, n := "", 0
	for k, v := range e.Attrs {
		if s, ok := v.(string); ok && len(s) > n {
			longest, n = k, len(s)
		}
	}
	if len(e.Msg) > n && len(e.Msg) > 16 {
		e.Msg = cut(e.Msg, len(e.Msg)/2)
		return true
	}
	if n > 16 {
		e.Attrs[longest] = cut(e.Attrs[longest].(string), n/2)
		return true
	}
	return false
}

func cut(s string, n int) string {
	for n > 0 && n < len(s) && !isRuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// DayFile is the first file of a local day in the store.
func DayFile(dir, day string) string { return filepath.Join(dir, day+".jsonl") }
