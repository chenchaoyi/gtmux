package dispatch

import (
	"strings"
)

// State is the deterministic outcome of a delivery.
type State string

const (
	StateLanded     State = "landed"            // confirmed in the conversation
	StateQueued     State = "queued"            // accepted, but behind the current turn
	StateFailed     State = "failed"            // not confirmed within the timeout
	StateRefusedDup State = "refused-duplicate" // identical payload re-sent inside the window
)

// Result is what Deliver returns. Delivered is true ONLY for StateLanded.
type Result struct {
	Delivered bool
	State     State
	Evidence  string // capture tail on failure, or a short note
	Attempts  int    // submit (Enter) attempts made
}

// Event kinds carried by IO.Events (mapped from the session-events stream).
const (
	EvSubmit  = "submit"  // UserPromptSubmit — Head is the prompt's normalized head
	EvStop    = "stop"    // Stop — the turn completed
	EvCompact = "compact" // PreCompact — compaction began
)

// Ev is one recent lifecycle event for the target pane, reduced to what verify
// needs (read from events.jsonl by the caller).
type Ev struct {
	Kind string
	Head string // normalized content head (EvSubmit)
	Ts   int64
}

// IO is the injectable I/O surface — real tmux/events in production, fakes in
// tests. Every field that Deliver may call must be non-nil except the optional
// ClearDraft / Events / RecentSend / RecordSend (guarded before use).
type IO struct {
	Capture    func() string            // full-screen capture (with scrollback margin)
	Paste      func(text string) error  // load-buffer + paste-buffer (no Enter)
	Enter      func() error             // submit
	ClearDraft func() error             // clear the input draft (C-u); optional
	InMode     func() bool              // pane is in tmux copy/view-mode (input swallowed); optional
	ExitMode   func() error             // drop out of copy/view-mode before typing; optional
	Events     func(sinceTs int64) []Ev // recent lifecycle events for this pane; optional
	Now        func() int64             // unix seconds (injectable clock)
	Sleep      func()                   // wait one poll interval (prod sleeps; test advances)
	RecentSend func(pane string) (string, int64)
	RecordSend func(pane, hash string, ts int64)
}

// Opts configures a delivery.
type Opts struct {
	Pane           string
	HookEquipped   bool  // prefer the deterministic event stream over the screen
	Force          bool  // override the re-send interlock
	ResendWindow   int64 // seconds; 0 disables the interlock
	DeliverTimeout int64 // seconds to confirm before giving up
	HookGrace      int64 // seconds to wait for a submit event before using the screen fallback
	PasteRetries   int   // extra paste attempts on a fragment
	EnterRetries   int   // extra Enter attempts on a swallowed submit
}

func (o *Opts) fillDefaults() {
	if o.DeliverTimeout <= 0 {
		o.DeliverTimeout = 15
	}
	if o.HookGrace <= 0 {
		o.HookGrace = 3
	}
	if o.PasteRetries < 0 {
		o.PasteRetries = 0
	}
	if o.EnterRetries <= 0 {
		o.EnterRetries = 3
	}
}

// Deliver pastes text into a pane and verifies it LANDED, layered: deterministic
// hook evidence first, a hardened two-frame screen-read as a fallback. See the
// hq-dispatch design for the full state machine.
func Deliver(io IO, opts Opts, text string) Result {
	opts.fillDefaults()
	head := NormalizeHead(text)

	// 1 · Re-send interlock (incident ⑨): refuse an identical payload within the window.
	if !opts.Force && io.RecentSend != nil &&
		isDuplicate(io.RecentSend, opts.Pane, text, io.Now(), opts.ResendWindow) {
		return Result{State: StateRefusedDup, Evidence: "identical payload re-sent within resendWindow"}
	}

	start := io.Now()

	// 2·3 · Paste with the fragment guard (incident ③).
	if !pasteWithGuard(io, opts, text) {
		return Result{State: StateFailed, Evidence: evidenceTail(io.Capture())}
	}
	if io.RecordSend != nil {
		io.RecordSend(opts.Pane, PayloadHash(opts.Pane, text), io.Now())
	}

	// 4 · Submit.
	attempts := 1
	_ = io.Enter()
	lastEnter := io.Now()

	// 5 · Verify loop.
	deadline := start + opts.DeliverTimeout
	var prevLanded, prevInDraft bool // two-frame consistency for the fallback
	first := true
	for {
		if !first {
			io.Sleep()
		}
		first = false

		// PRIMARY — deterministic hook evidence (no screen-read needed).
		if opts.HookEquipped && io.Events != nil {
			for _, e := range io.Events(start) {
				if e.Kind == EvSubmit && headsMatch(e.Head, head) {
					return Result{Delivered: true, State: StateLanded, Attempts: attempts}
				}
			}
		}

		screen := io.Capture()

		// A queued submission is a distinct, reported outcome (incident ④).
		if looksQueued(screen) {
			return Result{State: StateQueued, Attempts: attempts}
		}

		// FALLBACK — hardened, two-frame screen-read (hook-less agent, or the hook
		// stayed silent past the grace; the latter also covers a swallowed Enter for a
		// hook agent, since no submit event will ever arrive for an unsent draft).
		if !opts.HookEquipped || io.Now()-start >= opts.HookGrace {
			history, draft, _ := SplitInputRegion(screen)
			landed := !ContainsHead(draft, text) && ContainsHead(history, text)
			inDraft := draftHasDelivery(draft, text)
			// Only a verdict that AGREES with the previous frame is trusted (defeats the
			// single-frame ctx%/compact-bar misread, incident ⑩).
			if landed && prevLanded {
				return Result{Delivered: true, State: StateLanded, Attempts: attempts}
			}
			if inDraft && prevInDraft && attempts <= opts.EnterRetries &&
				io.Now()-lastEnter >= backoff(attempts) {
				_ = io.Enter() // swallowed Enter (incident ②) — re-submit with backoff
				attempts++
				lastEnter = io.Now()
			}
			prevLanded, prevInDraft = landed, inDraft
		}

		if io.Now() >= deadline {
			return Result{State: StateFailed, Evidence: evidenceTail(io.Capture()), Attempts: attempts}
		}
	}
}

// pasteWithGuard pastes text and confirms the FULL text (or a collapsed-paste
// placeholder) is in the draft, retrying a genuine fragment. Returns false if it
// cannot place the full text within PasteRetries.
//
// When the pane has NO locatable input region (structured == false, e.g. a plain
// shell prompt), it does NOT treat the empty draft as a fragment — clearing the
// draft (C-u) there would DESTROY the just-pasted text. It proceeds and lets
// post-submit verification decide.
func pasteWithGuard(io IO, opts Opts, text string) bool {
	for i := 0; i <= opts.PasteRetries; i++ {
		// Copy/view-mode swallows paste-buffer (and the later Enter) as mode-nav
		// commands (incident: a scrolled pane silently ate a whole dispatch). Drop out
		// of the mode BEFORE pasting so the payload actually reaches the input box.
		exitCopyMode(io)
		if err := io.Paste(text); err != nil {
			return false
		}
		_, draft, structured := SplitInputRegion(io.Capture())
		if draftHasDelivery(draft, text) {
			return true
		}
		if !structured {
			return true // can't see a draft to validate — don't destroy it; verify post-submit
		}
		if io.ClearDraft != nil {
			_ = io.ClearDraft() // clear the fragment before retrying
		}
	}
	return false
}

// exitCopyMode drops the pane out of tmux copy/view-mode before a write, but only
// when the injected IO can both sense and exit it (optional fields). A pane in a mode
// eats paste-buffer/Enter as navigation, so an un-cancelled scroll silently swallows
// a whole delivery — this makes the write land instead.
func exitCopyMode(io IO) {
	if io.InMode != nil && io.ExitMode != nil && io.InMode() {
		_ = io.ExitMode()
	}
}

// draftHasDelivery reports whether the draft holds the full delivery — either the
// literal text head, or a TUI's collapsed-paste placeholder ("[Pasted text +N
// lines]"), which stands in for a large paste the agent folded. A mere prefix (the
// "cl" fragment) matches neither.
func draftHasDelivery(draft, text string) bool {
	return ContainsHead(draft, text) || looksCollapsedPaste(draft)
}

// looksCollapsedPaste reports whether a draft shows a folded large-paste placeholder.
func looksCollapsedPaste(draft string) bool {
	low := strings.ToLower(draft)
	return strings.Contains(low, "[pasted text") || strings.Contains(low, "pasted text #")
}

// headsMatch compares two normalized heads tolerantly (equal, or one a prefix of
// the other) — the event head and the delivered head are the same content, so an
// exact match is expected; containment guards against a length mismatch.
func headsMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

// backoff returns the minimum seconds between the nth and (n+1)th Enter: 1, 2, 4,
// capped at 4 — a bounded exponential so a genuinely swallowed Enter recovers fast
// without hammering.
func backoff(attempt int) int64 {
	switch {
	case attempt <= 1:
		return 1
	case attempt == 2:
		return 2
	default:
		return 4
	}
}

// evidenceTail returns the last ~12 non-empty lines of a capture — the on-screen
// proof attached to a failed delivery so a caller (and HQ) can see what happened.
func evidenceTail(capture string) string {
	lines := strings.Split(strings.TrimRight(capture, "\n"), "\n")
	const n = 12
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
