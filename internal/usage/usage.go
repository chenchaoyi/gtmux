// Package usage is the usage-watch layer (see openspec usage-watch): per-session
// token accounting, layered per-agent-type thresholds, and ahead-of-time
// projections — all DETERMINISTIC (parsed from the agent's own session log,
// zero LLM calls, cgo-free), in keeping with the digest layer's cost model.
//
// Three facts per session, three different mechanics:
//   - context footprint: the LAST assistant message's input + cache_read +
//     cache_creation tokens ≈ the live context — exact, from the log tail.
//   - rate: output tokens/min over a recent sliding window — exact from the
//     tail (a 10-min window sits comfortably inside the tail bytes).
//   - cumulative burn: total output tokens — kept in a small persistent counter
//     (state/usage/<session>.json) updated INCREMENTALLY by byte offset, so no
//     caller ever re-scans a 100 MB log (the first-ever scan pays once).
package usage

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Session is one agent session's usage snapshot (the CLI/API/digest shape).
type Session struct {
	Agent      string  `json:"agent"` // agent key (claude/codex/…)
	SessionID  string  `json:"sessionId"`
	OutTok     int64   `json:"out"`                  // cumulative output tokens (persistent counter)
	InTok      int64   `json:"in"`                   // cumulative non-cached input tokens
	CtxTok     int64   `json:"ctx_tok"`              // live context footprint (last msg in+cache)
	CtxFrac    float64 `json:"ctx"`                  // CtxTok / the model window
	RatePerMin int64   `json:"rate"`                 // output tokens/min over the recent window
	LastAt     int64   `json:"last_at"`              // epoch seconds of the last usage message
	Warn       string  `json:"usage_warn,omitempty"` // first breached/projected layer, "" = fine
}

// rateWindow is the sliding window the rate is computed over.
const rateWindow = 10 * time.Minute

// tailBytes bounds the log tail read per call — plenty for the last message +
// a 10-minute window, tiny against a multi-GB log.
const tailBytes = 1 << 20 // 1 MiB

// ForSession computes a session's usage snapshot. ok=false when the agent has
// no log layout or the log doesn't exist (fields then mean nothing).
func ForSession(agent, sessionID string, now time.Time) (Session, bool) {
	path := transcript.LogPath(agent, sessionID)
	if path == "" {
		return Session{}, false
	}
	fi, err := os.Stat(path)
	if err != nil {
		return Session{}, false
	}
	s := Session{Agent: agent, SessionID: sessionID}

	// Cumulative counters: incremental by byte offset, persisted per session.
	c := loadCounter(sessionID)
	if c.Offset > fi.Size() { // log replaced/truncated → rescan from zero
		c = counter{}
	}
	grew, msgs := scanFrom(path, c.Offset, fi.Size())
	for _, m := range msgs {
		c.Out += m.out
		c.In += m.in
	}
	c.Offset = grew
	saveCounter(sessionID, c)
	s.OutTok, s.InTok = c.Out, c.In

	// Context + rate: from the tail (exact — the tail always contains the last
	// message; the window rate only counts what falls inside the window anyway).
	tail := tailMessages(path, fi.Size())
	if len(tail) > 0 {
		last := tail[len(tail)-1]
		s.CtxTok = last.in + last.cacheRead + last.cacheCreate
		s.LastAt = last.at.Unix()
		if w := windowFor(agent, last.model, s.CtxTok); w > 0 {
			s.CtxFrac = float64(s.CtxTok) / float64(w)
		}
		cut := now.Add(-rateWindow)
		var winOut int64
		var first time.Time
		for _, m := range tail {
			if m.at.Before(cut) {
				continue
			}
			if first.IsZero() {
				first = m.at
			}
			winOut += m.out
		}
		if !first.IsZero() {
			mins := now.Sub(first).Minutes()
			if mins < 1 {
				mins = 1
			}
			s.RatePerMin = int64(float64(winOut) / mins)
		}
	}
	return s, true
}

// counter is the persistent per-session cumulative record.
type counter struct {
	Offset int64 `json:"offset"` // bytes of the log already folded in
	Out    int64 `json:"out"`
	In     int64 `json:"in"`
}

// Dir holds the per-session usage counters.
func Dir() string { return filepath.Join(state.Dir(), "usage") }

func counterPath(sessionID string) string {
	return filepath.Join(Dir(), base64.RawURLEncoding.EncodeToString([]byte(sessionID))+".json")
}

func loadCounter(sessionID string) counter {
	var c counter
	if b, err := os.ReadFile(counterPath(sessionID)); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

func saveCounter(sessionID string, c counter) {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return
	}
	b, _ := json.Marshal(c)
	_ = os.WriteFile(counterPath(sessionID), b, 0o644)
}
