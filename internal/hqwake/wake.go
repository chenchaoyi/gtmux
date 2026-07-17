// Package hqwake is the deterministic WAKE side of HQ perception (hq-perception-v2):
// it builds the signal lines gtmux types into the HQ pane (the only knock), tracks
// the outcome tally behind the summary tick, and stamps/reads HQ's pull freshness.
//
// The split it implements: perception DATA stays pull-side (events/digest — silent,
// zero tokens), and AROUSAL is exactly one channel — a compact, visually-distinct
// wake line per decision-dense event plus a gated periodic tick. Everything here is
// plain code: no LLM, no tmux (delivery stays in hqnudge; callers wire the two).
package hqwake

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Wake classes — the ONLY event kinds allowed to type into the HQ pane
// (hq-wake-protocol). Everything else reaches HQ by pull.
const (
	ClassWaiting     = "waiting"
	ClassResolved    = "resolved"
	ClassAsks        = "asks"
	ClassDone        = "done"
	ClassCrash       = "crash"
	ClassGoalChanged = "goal-changed"
	ClassNewSession  = "new-session"
	ClassReapSuggest = "reap-suggest"
	ClassTick        = "tick"
	// ClassStuckWaiting is the lifecycle watchdog's escalation: a pane that has been
	// waiting on the user past the timeout. Declared late — it was emitted for months as
	// a hand-built `[gtmux] stuck·waiting …` line, which no playbook taught and PriorityOf
	// could not even recognize as a wake class (it queued as a default-priority outcome
	// rather than the escalation it is).
	ClassStuckWaiting = "stuck·waiting"
)

// Wake classes raised outside this package's own vocabulary (built by the serve
// tick and the feed/wake watchdogs), listed here so the priority table is complete.
const (
	ClassResourceWarn = "resource·warn"
	ClassLimitsWarn   = "limits·warn"
	ClassUsageWarn    = "usage·warn"
	ClassFeedDegraded = "feed-degraded"
	ClassWakeDegraded = "wake-degraded"
)

// Sigil opens every injected line. U+00BB — Latin-1-safe, so it survives the
// hostile-locale class of mangling that once ate ✳ (see tmux-utf8 fix v0.11.3).
const Sigil = "»"

// Wake priorities (hq-wake-reliability): the delivery queue drains highest priority
// first, oldest-first within a priority, so a decision-dense knock never waits behind
// a backlog of standing warnings — and, at the queue cap, a standing warning (which
// re-fires on the next tick) is evicted rather than a goal-changed (which never does).
const (
	PriorityDecision = 0 // the user or an agent is blocked on this being seen
	PriorityOutcome  = 1 // something finished/appeared/left — judge it, but it keeps
	PriorityStanding = 2 // a standing condition that re-fires on its own cadence
)

// PriorityDefault is the priority of a line whose class is unknown — including a queue
// entry written by an older gtmux, whose filename carries no priority field at all.
const PriorityDefault = PriorityOutcome

// classPriority is the wake-class → priority table. A class absent from it takes
// PriorityDefault.
var classPriority = map[string]int{
	ClassWaiting:      PriorityDecision,
	ClassAsks:         PriorityDecision,
	ClassGoalChanged:  PriorityDecision,
	ClassCrash:        PriorityDecision,
	ClassFeedDegraded: PriorityDecision,
	ClassWakeDegraded: PriorityDecision,
	ClassDone:         PriorityOutcome,
	ClassResolved:     PriorityOutcome,
	ClassNewSession:   PriorityOutcome,
	ClassReapSuggest:  PriorityOutcome,
	ClassTick:         PriorityOutcome,
	ClassStuckWaiting: PriorityDecision,
	ClassResourceWarn: PriorityStanding,
	ClassLimitsWarn:   PriorityStanding,
	ClassUsageWarn:    PriorityStanding,
}

// PriorityOf reads a wake line's class out of its own `» gtmux·<class>` prefix and
// returns that class's delivery priority. Parsing the line (rather than threading a
// priority through every call site) keeps the wake vocabulary in ONE place — this
// table — and means a line built anywhere is queued correctly. A `waiting·permission`
// class matches on its `waiting` stem.
func PriorityOf(line string) int {
	head := strings.TrimPrefix(strings.TrimSpace(line), Sigil)
	head = strings.TrimSpace(head)
	if !strings.HasPrefix(head, "gtmux·") {
		return PriorityDefault
	}
	class := strings.TrimPrefix(head, "gtmux·")
	if i := strings.IndexAny(class, " \t"); i >= 0 {
		class = class[:i]
	}
	if p, ok := classPriority[class]; ok {
		return p
	}
	if stem, _, found := strings.Cut(class, "·"); found { // waiting·permission → waiting
		if p, ok := classPriority[stem]; ok {
			return p
		}
	}
	return PriorityDefault
}

// sep separates the columnar fields. U+2502 (box drawing) reads as a column rule
// and is pinned by the format fixture test.
const sep = " │ "

// Line builds one wake line: `» gtmux·<class>  <head> │ f1 │ f2 …`. Empty fields
// are skipped so callers can pass optionals unconditionally. head is typically
// `<loc> (<pane>)`; agent/user-authored payloads must already be DATA-labelled
// (goal:"…" / title:"…" / tail:"…") by the caller.
func Line(class, head string, fields ...string) string {
	var b strings.Builder
	b.WriteString(Sigil)
	b.WriteString(" gtmux·")
	b.WriteString(class)
	if head = strings.TrimSpace(head); head != "" {
		b.WriteString("  ")
		b.WriteString(head)
	}
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			b.WriteString(sep)
			b.WriteString(f)
		}
	}
	return b.String()
}

// dir is hqwake's private state home (markers + tally) under the gtmux state dir.
func dir() string { return filepath.Join(state.Dir(), "hqwake") }

// ── done per-pane merge window ────────────────────────────────────────────────

// doneLastPath stamps the last immediate done-wake delivery per pane.
func doneLastPath(pane string) string { return filepath.Join(dir(), "done-last-"+pane) }

// DoneDue reports whether an immediate done wake for pane is outside the merge
// window (true → deliver now + stamp), or inside it (false → the caller queues a
// keyed replacement due at the returned time). now/gap in unix seconds.
func DoneDue(pane string, now, gapSec int64) (due bool, dueAt int64) {
	fi, err := os.Stat(doneLastPath(pane))
	if err != nil || now-fi.ModTime().Unix() >= gapSec {
		return true, now
	}
	return false, fi.ModTime().Unix() + gapSec
}

// StampDone records a delivered immediate done wake for the merge window.
func StampDone(pane string, now int64) {
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return
	}
	p := doneLastPath(pane)
	_ = os.WriteFile(p, nil, 0o644)
	t := time.Unix(now, 0)
	_ = os.Chtimes(p, t, t)
}

// ── pull freshness (consumer-side, replaces producer-heartbeat suppression) ──

// pullStampPath records when HQ last pulled the stream (events --since-seq /
// digest run from the HQ home).
func pullStampPath() string { return filepath.Join(dir(), "last-pull") }

// StampPull records an HQ-side pull now. Called by the CLI when a delta-read runs
// with the HQ home as cwd (the same cwd-keyed role rule the radar uses).
func StampPull() {
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(pullStampPath(), nil, 0o644)
	now := time.Now()
	_ = os.Chtimes(pullStampPath(), now, now)
}

// pullStaleSecs is the staleness threshold: past it, wake lines carry a catch-up
// hint. Generous — its job is nudging a brain that stopped pulling, not pacing one.
const pullStaleSecs = 30 * 60

// PullHint returns a pull-overdue hint field when HQ hasn't pulled within the
// staleness threshold ("" when fresh). sinceSeq is the suggested catch-up cursor;
// pass 0 when no cursor is known — the hint then recommends a full digest
// reconcile instead of an events replay.
func PullHint(now, sinceSeq int64) string {
	fi, err := os.Stat(pullStampPath())
	if err == nil && now-fi.ModTime().Unix() < pullStaleSecs {
		return ""
	}
	if sinceSeq <= 0 {
		return "pull overdue — reconcile: gtmux digest --json"
	}
	return fmt.Sprintf("pull overdue — run: gtmux events --since-seq %d --json", sinceSeq)
}
