// Package hqnudge delivers a compact event line into the live gtmux HQ pane WITHOUT
// ever clobbering or auto-submitting a half-typed draft the user is composing there.
//
// The bug it fixes: nudges were injected with `send-keys … Enter`. If the user was
// mid-typing in HQ when a nudge fired, the nudge text concatenated onto the draft AND
// the trailing Enter submitted the user's half-written command. Data loss.
//
// The guard: before typing, read the HQ input box (reusing the #393 dispatch region
// detector via dispatch.DraftOf) and check the pane isn't in tmux copy-mode. Deliver
// ONLY when the box is confirmed empty over TWO frames a short interval apart and the
// pane is not scrolling; otherwise the nudge is queued to disk and NOTHING is typed. A
// queued nudge is flushed (coalesced) on the next empty box — the next injection
// attempt, HQ's own turn-end (Stop, box reliably empty), or the serve tick.
//
// INVARIANT: no code path here sends Enter into a non-empty HQ input box.
//
// The hook is a short-lived process per event, so the queue is PERSISTENT (a dir of
// one-file-per-nudge under state.Dir()), not in-memory. Draining claims each file with
// an atomic rename so two concurrent hooks can't double-deliver one nudge.
package hqnudge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// twoFrameGap is the interval between the two empty-box reads. It shrinks (cannot
// eliminate) the window where the user starts typing between the check and the send.
const twoFrameGap = 300 * time.Millisecond

// queueDir holds one file per pending HQ nudge (`<nanos>-<pid>.txt`).
func queueDir() string { return filepath.Join(state.Dir(), "hq-nudges") }

// io is the injectable I/O surface — real tmux in production, fakes in tests.
type io struct {
	capture func(pane string) string // visible-screen capture (the input box is at its foot)
	send    func(pane, text string) error
	sleep   func() // wait the two-frame interval
	nowNano func() int64
	inMode  func(pane string) bool // pane is in tmux copy/view-mode (scrolling)
}

var prod = io{
	capture: tmux.CapturePane,
	send:    func(pane, text string) error { return tmux.SendText(pane, text, true) },
	sleep:   func() { time.Sleep(twoFrameGap) },
	nowNano: func() int64 { return time.Now().UnixNano() },
	inMode:  func(pane string) bool { return tmux.Display(pane, "#{pane_in_mode}") == "1" },
}

// Deliver types msg into the HQ pane, guarding a half-typed draft. msg is queued to
// disk first; if the HQ box is confirmed empty over two frames, the queue (msg plus
// any backlog) is flushed coalesced. If the box holds a draft, msg stays queued and
// NOTHING is typed. Public entry — every HQ injection goes through here.
func Deliver(pane, msg string) { deliver(prod, pane, msg) }

func deliver(x io, pane, msg string) {
	if pane == "" || strings.TrimSpace(msg) == "" {
		return
	}
	enqueue(x, msg) // to disk; nothing typed yet
	if boxEmpty(x, pane) {
		drainInto(x, pane) // one send, coalesced — this is the ONLY place we type
	}
}

// Drain flushes any queued nudges into the HQ pane when its box is empty (coalesced).
// Called on HQ's own turn-end (box reliably empty) and on the serve tick, so a queued
// nudge lands even with no further Deliver call. No-op on a drafted box or empty queue.
func Drain(pane string) { drain(prod, pane) }

func drain(x io, pane string) {
	if pane == "" || !hasPending() {
		return // cheap gate: skip the capture/sleep when nothing is queued
	}
	if boxEmpty(x, pane) {
		drainInto(x, pane)
	}
}

// Pending reports whether any nudge is queued behind a draft. Cheap (a dir scan, no
// tmux) so callers can gate the more expensive Drain (which captures the pane) on it.
func Pending() bool { return hasPending() }

func hasPending() bool {
	entries, _ := os.ReadDir(queueDir())
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			return true
		}
	}
	return false
}

// boxEmpty reports whether the HQ input box is empty, confirmed over TWO frames. A
// non-empty first frame returns false immediately (no sleep, no send). A capture with
// no locatable input region (structured == false) is treated as NOT empty — we only
// ever type into a confirmed-empty, structured box.
func boxEmpty(x io, pane string) bool {
	// Copy/view-mode: the user is scrolling, and injected keys are eaten as copy-mode
	// NAV commands (`f` → jump-forward, yellow residue) instead of reaching the box.
	// Treat it exactly like a non-empty draft — do not inject; queue, deliver on exit.
	if x.inMode != nil && x.inMode(pane) {
		return false
	}
	if draft, structured := dispatch.DraftOf(x.capture(pane)); !structured || strings.TrimSpace(draft) != "" {
		return false
	}
	x.sleep()
	draft, structured := dispatch.DraftOf(x.capture(pane))
	return structured && strings.TrimSpace(draft) == ""
}

// enqueue writes one pending-nudge file. The name is time-ordered (fixed-width nanos)
// so a lexical sort on drain is oldest-first; the pid suffix avoids collisions between
// concurrent hook processes.
func enqueue(x io, msg string) {
	if err := os.MkdirAll(queueDir(), 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("%019d-%d.txt", x.nowNano(), os.Getpid())
	_ = os.WriteFile(filepath.Join(queueDir(), name), []byte(msg), 0o644)
}

// drainInto delivers the queue as ONE coalesced line. It MUST be called only after
// boxEmpty confirmed an empty box (the invariant). Each file is claimed by an atomic
// rename to `.sending` so a concurrent drainer can't re-deliver it; a lost claim just
// skips that file (at worst a coalesce splits across two lines — never a loss/dup).
func drainInto(x io, pane string) {
	entries, _ := os.ReadDir(queueDir())
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // oldest-first (fixed-width nanos prefix)

	var claimed, msgs []string
	for _, n := range names {
		src := filepath.Join(queueDir(), n)
		dst := src + ".sending"
		if os.Rename(src, dst) != nil {
			continue // lost the claim to another drainer
		}
		claimed = append(claimed, dst)
		if b, err := os.ReadFile(dst); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				msgs = append(msgs, s)
			}
		}
	}
	if len(msgs) > 0 {
		_ = x.send(pane, strings.Join(msgs, " · "))
	}
	for _, c := range claimed {
		_ = os.Remove(c)
	}
}
