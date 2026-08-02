package dispatch

import (
	"strings"
	"unicode"
)

// State is the deterministic outcome of a delivery.
type State string

const (
	StateLanded     State = "landed"            // confirmed in the conversation
	StateQueued     State = "queued"            // accepted, but behind the current turn
	StateFailed     State = "failed"            // not confirmed within the timeout
	StateRefusedDup State = "refused-duplicate" // identical payload re-sent inside the window
)

// JudgedBy values — which evidence layer decided a delivery's outcome. Attributed
// so a misjudgment can be pinned to its layer instead of reconstructed from
// timelines (openspec change agent-drivers).
const (
	JudgedByDriver = "driver" // deterministic agent evidence (submit event on the stream)
	JudgedByScreen = "screen" // Layer-1 screen read (two-frame fallback / timeout)
)

// Result is what Deliver returns. Delivered is true ONLY for StateLanded.
type Result struct {
	Delivered bool
	State     State
	Evidence  string // capture tail on failure, or a short note
	Attempts  int    // submit (Enter) attempts made
	JudgedBy  string // JudgedByDriver | JudgedByScreen ("" when refused before judging)
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
	PasteSettle    int   // extra frames to let a paste render before calling it a fragment
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
	if o.PasteSettle <= 0 {
		o.PasteSettle = 3
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
	// The event needle rides the SAME pipeline as the hook-recorded Summary
	// (NormalizeNeedle) so a cleaning difference can never make the verifier
	// ignore a genuine submit event. Screen comparisons keep using the raw text —
	// the screen shows what was pasted, uncleaned.
	head := NormalizeNeedle(text)

	// 1 · Re-send interlock (incident ⑨): refuse an identical payload within the window.
	if !opts.Force && io.RecentSend != nil &&
		isDuplicate(io.RecentSend, opts.Pane, text, io.Now(), opts.ResendWindow) {
		return Result{State: StateRefusedDup, Evidence: "identical payload re-sent within resendWindow"}
	}

	start := io.Now()

	// 2·3 · Paste with the fragment guard (incident ③).
	//
	// For a HOOK-EQUIPPED agent we DO NOT abort on the guard's "settled fragment" verdict:
	// on a busy pane that verdict is usually a redraw race where the FULL paste actually
	// landed, and letting the fragile pre-submit screen scrape be the final word is the
	// recurring false "input box didn't confirm the full message". Submit anyway and let the
	// deterministic receipt below judge — the UserPromptSubmit event is matched on the FULL
	// needle, so a complete paste confirms (StateLanded) and a genuinely truncated one won't
	// match (StateFailed). A hook-LESS agent has no receipt, so the draft scrape stays its
	// only confirmation and a settled fragment must still fail (submitting a known-truncated
	// draft is worse than reporting it).
	if !pasteWithGuard(io, opts, text) && !opts.HookEquipped {
		return Result{State: StateFailed, Evidence: evidenceTail(io.Capture()), JudgedBy: JudgedByScreen}
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
		if submitConfirmed(io, opts, head, start) {
			return Result{Delivered: true, State: StateLanded, Attempts: attempts, JudgedBy: JudgedByDriver}
		}

		screen := io.Capture()

		// A queued submission is a distinct, reported outcome (incident ④).
		if looksQueued(screen) {
			return Result{State: StateQueued, Attempts: attempts, JudgedBy: JudgedByScreen}
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
				return Result{Delivered: true, State: StateLanded, Attempts: attempts, JudgedBy: JudgedByScreen}
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
			// FINAL receipt re-check (invariant I2): a submit event that arrived between
			// the last poll and the deadline must not be lost to the timeout — this was a
			// real misjudgment source ("NOT delivered" reported while UserPromptSubmit sat
			// in the stream). Only after this re-read may the screen's failure verdict
			// stand; a stream-confirmed landing is final.
			if submitConfirmed(io, opts, head, start) {
				return Result{Delivered: true, State: StateLanded, Attempts: attempts, JudgedBy: JudgedByDriver}
			}
			return Result{State: StateFailed, Evidence: evidenceTail(io.Capture()), Attempts: attempts, JudgedBy: JudgedByScreen}
		}
	}
}

// submitConfirmed scans the pane's event stream for a submit event whose recorded
// head matches the delivery needle — the deterministic receipt that both the verify
// loop and the pre-failure final re-check consult. False when the event channel is
// absent or silent, which is NEVER failure evidence — the judgment falls to the
// screen (I2).
func submitConfirmed(io IO, opts Opts, head string, since int64) bool {
	if !opts.HookEquipped || io.Events == nil {
		return false
	}
	for _, e := range io.Events(since) {
		if e.Kind == EvSubmit && HeadsMatch(e.Head, head) {
			return true
		}
	}
	return false
}

// PasteAndSubmit pastes text into a pane's input draft (with the same fragment guard
// the verified dispatch uses), confirms the FULL payload is present, THEN sends Enter
// once. It is the paste→confirm→submit core shared by the UNVERIFIED send paths —
// `POST /api/send` (phone / menu-bar reply) and `gtmux send --no-verify` — so they no
// longer race a blind Enter against a still-rendering paste (the truncation / swallowed-
// Enter bug). It deliberately does NOT run the post-submit LANDED-verification loop:
// those paths stay fast on the phone's latency budget, differing from verified dispatch
// only in whether they confirm the landing AFTER submit — not in whether they confirm
// the DRAFT before it.
//
// Enter is sent whenever the paste was PLACED — a confirmed full draft, or a pane with
// no locatable input region (a plain shell, where the draft can't be validated and a
// bare command must still submit). It is WITHHELD only when the guard positively
// settled on a fragment it could not place in full: submitting a known-truncated draft
// is exactly what this fixes. Returns whether the full draft was confirmed.
func PasteAndSubmit(io IO, opts Opts, text string) bool {
	opts.fillDefaults()
	if !pasteWithGuard(io, opts, text) {
		return false // a settled fragment — do not submit a truncated draft
	}
	_ = io.Enter()
	return true
}

// pasteWithGuard puts text in the pane's input draft and confirms the FULL text (or
// a collapsed-paste placeholder) is there, retrying a genuine fragment. Returns
// false if it cannot place the full text within PasteRetries.
//
// The guard is IDEMPOTENT: the same text is never pasted twice into the same draft.
// It used to be — a stale frame or an unconfirmed clear sent it round the loop, and
// with PasteRetries=2 one instruction could be pasted three times, concatenated into
// the box and submitted as that mess. Two rules make a duplicate impossible: a
// re-paste happens only after the draft is CONFIRMED free of the last attempt's text
// (see clearedForRetry), and a draft that already holds the delivery is left alone.
//
// When the pane has NO locatable input region (structured == false, e.g. a plain
// shell prompt), it does NOT treat the empty draft as a fragment — clearing the
// draft (C-u) there would DESTROY the just-pasted text. It proceeds and lets
// post-submit verification decide.
func pasteWithGuard(io IO, opts Opts, text string) bool {
	for attempt := 0; ; attempt++ {
		// A retry re-reads FIRST: if the last attempt's paste is now in the draft (it
		// only rendered late), it is already delivered and pasting again would duplicate it.
		if attempt > 0 && draftHolds(io, text) {
			return true
		}
		// Copy/view-mode swallows paste-buffer (and the later Enter) as mode-nav
		// commands (incident: a scrolled pane silently ate a whole dispatch). Drop out
		// of the mode BEFORE pasting so the payload actually reaches the input box.
		exitCopyMode(io)
		if err := io.Paste(text); err != nil {
			return false
		}
		switch confirmPaste(io, opts, text) {
		case pasteInDraft, pasteUnverifiable:
			return true
		}
		// A settled fragment. Retry only while a retry can't duplicate anything.
		if attempt >= opts.PasteRetries || !clearedForRetry(io, opts) {
			return false
		}
	}
}

// pasteVerdict is what confirmPaste concluded about the draft after a paste.
type pasteVerdict int

const (
	pasteInDraft      pasteVerdict = iota // the full delivery is in the draft
	pasteFragment                         // the draft settled on less than the delivery
	pasteUnverifiable                     // no locatable draft — nothing to validate against
)

// confirmPaste waits for a paste to RENDER in the draft. paste-buffer returns as soon
// as tmux has written the bytes to the pty, but the agent TUI redraws on its own
// schedule and a loaded pane can be a frame behind — so the frame right after a paste
// is not proof of anything. A positive is trusted as soon as it appears (the common
// case, and it keeps a healthy send fast); "fragment" is the verdict that authorizes
// destroying and re-pasting, so it is returned only after the whole settle window has
// passed with no match.
func confirmPaste(io IO, opts Opts, text string) pasteVerdict {
	budget := settleFrames(opts.PasteSettle, text)
	// A BUSY pane (an agent mid-render) draws a long paste PROGRESSIVELY: the draft
	// fills in frame by frame while the agent's own output competes for redraws. A fixed
	// window read the half-rendered draft, called it a fragment, and CLEARED it — which
	// truncated a send into a churning pane and refused to submit (the "into a working
	// pane, only the first words land" report). So while the draft is still GROWING toward
	// the target, keep waiting; declare a fragment only once BOTH the settle budget has
	// passed AND the draft has STALLED (stopped advancing) for a few frames — with a hard
	// ceiling so a pathological pane can't stall a send forever.
	prevLen, stall := -1, 0
	ceiling := budget + pasteStallCeiling
	for i := 0; ; i++ {
		_, draft, structured := SplitInputRegion(io.Capture())
		if !structured {
			return pasteUnverifiable
		}
		if draftHasDelivery(draft, text) {
			return pasteInDraft
		}
		if n := len(normalizeSpace(draft)); n > prevLen {
			prevLen, stall = n, 0 // still arriving — do not clear a paste mid-render
		} else {
			stall++
		}
		if i >= ceiling || (i >= budget && stall >= pasteStallFrames) {
			return pasteFragment
		}
		io.Sleep()
	}
}

// settleFrames is how long to let a paste RENDER before calling it a fragment, scaled
// by how much was pasted.
//
// The fixed budget was the bug: PasteSettle is 3 frames (~900ms) whether the payload is
// "1" or three paragraphs. A TUI redraws a long paste progressively, so a big message
// could still be arriving when the deadline passed — the guard read the partial draft,
// called it a fragment, and (correctly, by its own rule) refused to press Enter. The
// user saw exactly that: a long message landing with its tail missing and no submit.
// "Sometimes", because it is a race with rendering.
//
// Scaling only ever makes us WAIT LONGER before giving up: confirmPaste still returns
// the instant the draft holds the delivery, so a short paste is as fast as it ever was,
// and nothing is submitted that would not have been submitted before.
func settleFrames(base int, text string) int {
	if base <= 0 {
		base = 1
	}
	// One extra frame per settleCharsPerFrame characters, capped so a pathological
	// payload can't stall a delivery for minutes.
	extra := len(text) / settleCharsPerFrame
	if extra > settleMaxExtraFrames {
		extra = settleMaxExtraFrames
	}
	return base + extra
}

const (
	// settleCharsPerFrame: how many pasted characters buy one more render frame. Sized
	// against a real terminal: a few hundred characters is roughly one more redraw.
	settleCharsPerFrame = 400
	// settleMaxExtraFrames caps the added wait (~6s at a 300ms poll) — past that the
	// draft is not merely slow, and the caller is better served by a reported failure
	// than an unbounded stall.
	settleMaxExtraFrames = 20
	// pasteStallFrames: consecutive frames with NO further draft growth before a
	// still-unmatched draft is called a fragment (a busy pane renders progressively, so
	// a couple of static frames — not one — is the real "it stopped arriving" signal).
	pasteStallFrames = 3
	// pasteStallCeiling caps how long the growth-tracking can extend the settle window
	// beyond the base budget (~6s more at a 300ms poll), so a pane that dribbles output
	// forever still fails in bounded time rather than hanging the send.
	pasteStallCeiling = 20
)

// clearedForRetry clears a fragmented draft and reports whether the box is now
// demonstrably EMPTY — the only state in which re-pasting cannot duplicate anything,
// because a paste appends to whatever the box already holds.
//
// The bar is this high because ClearDraft (C-u) is not the "empty the box" it was
// assumed to be. On a real Claude Code pane it kills only the line the cursor sits
// on: a C-u against a three-line draft leaves two lines, and a second C-u (or
// Escape) against what's left does nothing at all. The old guard re-pasted without
// looking — so the retry CONCATENATED the instruction onto the leftover, which is how
// one dispatched message ended up on screen two and three times. An unclearable draft
// now fails the delivery, with the box in the evidence for the caller to read.
func clearedForRetry(io IO, opts Opts) bool {
	if io.ClearDraft == nil {
		return false
	}
	_ = io.ClearDraft()
	for i := 0; ; i++ {
		_, draft, structured := SplitInputRegion(io.Capture())
		if !structured || normalizeSpace(draft) == "" {
			return true
		}
		if i >= opts.PasteSettle {
			return false
		}
		io.Sleep()
	}
}

// draftHolds reports whether the pane's draft already holds the delivery.
func draftHolds(io IO, text string) bool {
	_, draft, structured := SplitInputRegion(io.Capture())
	return structured && draftHasDelivery(draft, text)
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

// draftHasDelivery reports whether the draft holds the FULL delivery — either both
// the leading fingerprint (head) AND the trailing fingerprint (tail) of the literal
// text, or a TUI's collapsed-paste placeholder ("[Pasted text +N lines]"), which
// stands in for a large paste the agent folded. A head-only match is NOT enough: a
// long/multi-line paste can render its head a frame before its tail, and submitting
// on the head alone sends a truncated draft that the fallback then misreads as
// landed (the "task tail severed" bug). Requiring the tail too waits the paste out.
// A mere prefix (the "cl" fragment) matches neither head nor tail. Because this same
// predicate gates the swallowed-Enter re-submit, a draft that has been submitted
// (now empty) or mangled no longer satisfies it — so Enter is never re-sent blindly.
func draftHasDelivery(draft, text string) bool {
	if looksCollapsedPaste(draft) {
		return true
	}
	if ContainsHead(draft, text) && ContainsTail(draft, text) {
		return true
	}
	// Attachment-aware: a pasted image PATH lands in one of two shapes. Claude Code
	// usually FOLDS it into an "[Image #N]" chip, so the path text is never in the draft
	// verbatim (the "uploaded image never sends" bug). But it also sometimes keeps the
	// path LITERAL — and a long path WRAPS in the input box, so normalizeSpace injects a
	// space the path never had and the whole-text head/tail check above misses its tail
	// (the "text + image, everything is in the box but Not-sent" report). So when the
	// only lines the head/tail match is missing are image paths, the images count as
	// placed if the draft shows a chip OR holds every path literally (matched
	// whitespace-free, wrap-tolerant); any remaining prose must still match head+tail.
	rest, images := stripImagePathLines(text)
	if len(images) == 0 || !(looksAttachmentChip(draft) || draftHoldsImagePaths(draft, images)) {
		return false
	}
	if strings.TrimSpace(rest) == "" {
		return true
	}
	return ContainsHead(draft, rest) && ContainsTail(draft, rest)
}

// looksCollapsedPaste reports whether a draft shows a folded large-paste placeholder.
func looksCollapsedPaste(draft string) bool {
	low := strings.ToLower(draft)
	return strings.Contains(low, "[pasted text") || strings.Contains(low, "pasted text #")
}

// looksAttachmentChip reports whether the draft shows an attachment placeholder —
// what Claude Code renders in place of a pasted image path ("[Image #1]").
func looksAttachmentChip(draft string) bool {
	return strings.Contains(strings.ToLower(draft), "[image #")
}

// imageExts are the path suffixes an agent TUI folds into an attachment chip on
// paste — the formats the phone's photo/screenshot flow produces.
var imageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".heic", ".heif"}

// stripImagePathLines splits the lines that are bare image paths (the shape the mobile
// composer appends after the typed body: one absolute path per line) from the rest of
// the prose, returning the prose and the image paths (nil = none).
func stripImagePathLines(text string) (rest string, images []string) {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if isImagePathLine(line) {
			images = append(images, strings.TrimSpace(line))
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n"), images
}

// draftHoldsImagePaths reports whether every image path is present in the draft as
// LITERAL text (Claude keeps a pasted path literal on some panes instead of folding it
// into an [Image #N] chip). Matched WHITESPACE-FREE: a long path WRAPS in the input box
// and normalizeSpace turns the wrap into a space the path never had — stripping all
// whitespace from both sides recovers the wrapped path (paths have no interior spaces).
func draftHoldsImagePaths(draft string, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	ns := removeAllSpace(draft)
	for _, p := range paths {
		if !strings.Contains(ns, removeAllSpace(p)) {
			return false
		}
	}
	return true
}

// removeAllSpace strips every whitespace rune (not merely collapsing runs, as
// normalizeSpace does) — for matching a token that must not contain spaces (a path)
// against a draft where a line wrap injected one.
func removeAllSpace(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isImagePathLine reports whether a single line is nothing but a path to an image
// file — no spaces beyond the path itself, an image extension at the end.
func isImagePathLine(line string) bool {
	t := strings.ToLower(strings.TrimSpace(line))
	if t == "" || strings.ContainsAny(t, " \t") {
		return false
	}
	for _, ext := range imageExts {
		if strings.HasSuffix(t, ext) {
			return true
		}
	}
	return false
}

// HeadsMatch compares two normalized heads tolerantly (equal, or one a prefix of
// the other) — the event head and the delivered head are the same content, so an
// exact match is expected; containment guards against a length mismatch. Exported
// because the driver receipt (internal/driver) matches event summaries against a
// delivery needle with the same rule Deliver uses.
func HeadsMatch(a, b string) bool {
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
