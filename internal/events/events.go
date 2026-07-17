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
	Ts int64 `json:"ts"` // unix seconds
	// Seq is a strictly increasing sequence number assigned at the single append
	// path (hq-attention-system): it gives consumers a total order and a durable
	// cursor position that survives rotation (byte offsets do not). Additive — a
	// legacy record without it reads as sequence-unknown (0) and is ordered by ts.
	Seq     int64  `json:"seq,omitempty"`
	Event   string `json:"event"`          // Stop | Waiting | Notification | UserPromptSubmit | PreCompact | …
	State   string `json:"state"`          // derived: working | waiting | idle | …
	Pane    string `json:"pane,omitempty"` // tmux pane id ("" for native)
	Loc     string `json:"loc,omitempty"`
	Session string `json:"session,omitempty"`
	Agent   string `json:"agent,omitempty"`
	Kind    string `json:"kind,omitempty"` // waiting kind: permission | plan | question
	// Additive (hq-dispatch): a short content summary — the reply tail on Stop, the
	// prompt's normalized head on UserPromptSubmit (so dispatch verify can match a
	// submission deterministically from the stream, not by screen-reading).
	Summary string `json:"summary,omitempty"`
	// Additive (hq-dispatch): a deterministic turn-end classification on Stop —
	// "asking" (the reply ends on a question to the user) or "report". Empty otherwise.
	Class string `json:"class,omitempty"`
	// Additive (hq-attention-stream): on a prompt submission, whether the payload is a
	// real INSTRUCTION ("instruction") rather than harness-injected content or gtmux's
	// own wake line echoed back. Empty otherwise, and on a legacy record.
	//
	// It deliberately does NOT say WHO authored the instruction: a task `gtmux spawn`
	// delivered carries one too, and no reliable signal separates them (the send side
	// records a payload hash, while the harness appends reminder blocks to the prompt it
	// reports — so the hash misses exactly when it would matter). An instruction reaching
	// a session is a fleet change either way, and when in doubt the classifier must guess
	// "instruction": a duplicate costs a line, a miss costs the user's words.
	Origin string `json:"origin,omitempty"`
	// Additive (hq-chief-of-staff): a deterministic attention tier —
	// "routine" | "notable" | "important" — stamped at the source (Append) so a
	// supervisor can triage without the raw full text. Absent on a legacy record, which
	// reads as "routine".
	Severity string `json:"severity,omitempty"`
}

// OriginInstruction marks a prompt submission whose payload is a real instruction —
// typed prose or a slash command. See Record.Origin: author-agnostic by design.
const OriginInstruction = "instruction"

// Severity levels (attention tiers), lowest → highest.
const (
	SevRoutine   = "routine"
	SevNotable   = "notable"
	SevImportant = "important"
)

// SeverityRank orders the tiers for "this level and above" filtering; an
// unrecognized/empty level ranks as routine (0).
func SeverityRank(level string) int {
	switch level {
	case SevNotable:
		return 1
	case SevImportant:
		return 2
	default:
		return 0 // routine, and any legacy/empty value
	}
}

// Severity classifies a record's attention tier deterministically (no LLM) from
// fields the record already carries. It ranks URGENCY — how much someone is waiting on
// the supervisor — NOT relevance: a Waiting (the pane needs the user), an "asking"
// turn-end, and a crashed turn (StopFailure — the turn died on an agent/API error, which
// must never read as a finish) are important. A "report" turn-end, the session lifecycle
// events, and a submission that carries an INSTRUCTION are notable — they change the
// picture without anyone being blocked. Everything else is routine.
//
// A submitted instruction was routine until hq-attention-stream, while the playbook
// called `--severity important` "the attention stream" — so an HQ following its own
// playbook could not see what the user told a session to do. Severity is pure, so the
// verdict has to arrive on the record (Origin), stamped by the hook's own classifier.
func Severity(r Record) string {
	switch r.Event {
	case "Waiting":
		if r.Kind != "" {
			return SevImportant
		}
		return SevNotable
	case "Stop":
		if r.Class == "asking" {
			return SevImportant
		}
		return SevNotable
	case "StopFailure":
		return SevImportant
	case "SessionStart", "SessionEnd", "Resumed", "PreCompact":
		return SevNotable
	case "UserPromptSubmit":
		if r.Origin == OriginInstruction {
			return SevNotable // an instruction reached a session — a fleet change
		}
		return SevRoutine // harness injection, an echoed wake line: not an act
	default:
		return SevRoutine
	}
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
	// Stamp the attention tier at the source so it is persisted and queryable
	// without recompute; leave an explicitly-set value untouched (future-proofing).
	if r.Severity == "" {
		r.Severity = Severity(r)
	}
	// Assign the monotonic sequence at the source (hq-attention-system) so every
	// event carries a durable, rotation-independent cursor position. Leave an
	// explicitly-set value untouched. A 0 (counter unavailable) reads as
	// sequence-unknown downstream — never fatal.
	if r.Seq == 0 {
		r.Seq = nextSeq()
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

// OverCeiling reports whether the active log has grown past ~2× its rotation cap —
// a sign rotation is NOT firing (it should keep the active file under the cap). It is
// a cheap, LLM-free health probe for the self-check sensor. False when the log is
// absent or the cap is disabled.
func OverCeiling() bool {
	cap := capBytes()
	if cap <= 0 {
		return false
	}
	fi, err := os.Stat(Path())
	if err != nil {
		return false
	}
	return fi.Size() > 2*cap
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
