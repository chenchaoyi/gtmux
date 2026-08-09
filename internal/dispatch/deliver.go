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
	// StateRefusedDraft: the pane's input box already held UNSUBMITTED text that is not
	// ours. Pasting appends to a draft — it does not replace it — so delivering would
	// concatenate someone's half-written sentence onto the payload and submit both as one
	// message. The nudge channel has refused to type into a non-empty box since
	// hq-nudge-hardening; this is the same rule for the dispatch channel.
	StateRefusedDraft State = "refused-draft"
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
	Capture func() string // full-screen capture (with scrollback margin)
	// CaptureColor is the COLOR capture (`capture-pane -e`) the draft guard reads, so
	// Claude Code's FAINT suggested-next-command ghost text is excluded. Optional; falls
	// back to Capture, which cannot tell the ghost from a typed draft.
	CaptureColor func() string
	Paste        func(text string) error  // load-buffer + paste-buffer (no Enter)
	Enter        func() error             // submit
	ClearDraft   func() error             // clear the input draft (C-u); optional
	InMode       func() bool              // pane is in tmux copy/view-mode (input swallowed); optional
	ExitMode     func() error             // drop out of copy/view-mode before typing; optional
	Events       func(sinceTs int64) []Ev // recent lifecycle events for this pane; optional
	Now          func() int64             // unix seconds (injectable clock)
	Sleep        func()                   // wait one poll interval (prod sleeps; test advances)
	RecentSend   func(pane string) (string, int64)
	RecordSend   func(pane, hash string, ts int64)
	// ForgetSend drops a pane's interlock record. Called when a delivery ends FAILED, so
	// the interlock cannot refuse the retry of a send that never landed; optional.
	ForgetSend func(pane string)
}

// Opts configures a delivery.
type Opts struct {
	Pane         string
	HookEquipped bool // prefer the deterministic event stream over the screen
	Force        bool // override the re-send interlock
	// ClobberDraft overrides the DRAFT guard — deliver even when the input box holds
	// someone else's unsubmitted text. Deliberately NOT folded into Force: the phone sets
	// Force on every send (it carries its own sendID idempotency), and letting that also
	// waive draft protection would leave the surface most likely to clobber a draft — a
	// send from another device, into a pane the user may be typing in — unprotected.
	ClobberDraft bool
	// HasComposer says a KNOWN agent drives this pane, so it has a real input composer and
	// the text below it is a draft worth protecting. The draft guard runs ONLY then.
	//
	// Routing fails SAFE to the agent pipeline for anything that is neither a known agent
	// nor a bare shell — vim, ssh, a TUI app — and those panes have no composer at all. On
	// them SplitInputRegion's no-box degrade path locks onto whatever prompt-ish line is on
	// screen and reports the transcript as a "draft" (measured live: a pane running `diting`
	// yielded 347 characters of log output), which would refuse every send with a reason
	// that is not even true. Off by default, so a caller that has not thought about it gets
	// the pre-guard behavior rather than a mystery refusal.
	HasComposer    bool
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

	// 1b · Draft guard: never write into someone else's unsubmitted text.
	//
	// A paste APPENDS to the input box. If the user (or the agent's own prompt) has a
	// half-written line sitting there, delivering concatenates the payload onto it and
	// submits the pair as one message — the user's sentence is mangled and sent without
	// them ever pressing Enter. The nudge channel has refused a non-empty box since
	// hq-nudge-hardening; the dispatch channel never did, so `gtmux send` (CLI, HQ, and
	// the phone) could silently eat a draft.
	//
	// Refusing is the right answer here rather than queueing: unlike a wake, a send has a
	// caller waiting on a verdict, and a refusal it can see beats a message it cannot
	// take back. A draft that ALREADY holds this delivery is not someone else's text —
	// it is our own re-send landing idempotently, so it proceeds. `--force` overrides,
	// same as the interlock.
	if blocked, evidence := draftBlocked(io, opts, text); blocked {
		return Result{State: StateRefusedDraft, Evidence: evidence}
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
		return failed(io, opts, Result{State: StateFailed, Evidence: evidenceTail(io.Capture()), JudgedBy: JudgedByScreen})
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
			return failed(io, opts, Result{State: StateFailed, Evidence: evidenceTail(io.Capture()), Attempts: attempts, JudgedBy: JudgedByScreen})
		}
	}
}

// failed finalizes a FAILED delivery by dropping the pane's interlock record, so the
// obvious next move — send it again — is not refused.
//
// The interlock records the payload the moment the paste is placed, which is right: a
// crash between paste and submit must not let the same instruction be delivered twice. But
// it recorded "we tried this" and was read as "this was delivered", so a send that failed
// verification poisoned its own retry — `gtmux send` answered `refused-duplicate` to the
// one thing the operator obviously wanted, and `--force` was the only way through. Measured
// against a Codex whose receipt channel was dead, that was every retry.
//
// Only StateFailed forgets. StateQueued was ACCEPTED (it sits behind the current turn), and
// re-sending it would duplicate the instruction — exactly what the interlock exists for.
func failed(io IO, opts Opts, r Result) Result {
	if io.ForgetSend != nil {
		io.ForgetSend(opts.Pane)
	}
	return r
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
// Enter is sent whenever the paste was PLACED — a confirmed full draft, a pane with no
// locatable input region (a plain shell, where the draft can't be validated and a bare
// command must still submit), or a CHURNING pane whose box read never settled but whose
// non-empty draft shows the paste landed (a busy %pane; submitting best-effort beats
// silently dropping the send — the receipt still lands on the event stream). It is
// WITHHELD only when the guard positively settled on a fragment it could not place in
// full ON A QUIET pane: submitting a known-truncated draft is exactly what this fixes.
// Returns whether Enter was sent (true once the paste was placed by any of the above).
func PasteAndSubmit(io IO, opts Opts, text string) (ok bool, refused State) {
	opts.fillDefaults()
	// The UNVERIFIED path needs the draft guard just as much — more, in fact. It is what
	// `POST /api/send` uses, so it is a send from ANOTHER DEVICE into a pane whose owner
	// may be mid-sentence at the keyboard: the one case where nobody watching can undo it.
	if blocked, _ := draftBlocked(io, opts, text); blocked {
		return false, StateRefusedDraft
	}
	if !pasteWithGuard(io, opts, text) {
		return false, StateFailed // a settled fragment — do not submit a truncated draft
	}
	_ = io.Enter()
	return true, ""
}

// draftBlocked reports whether the pane's input box holds someone ELSE's unsubmitted text,
// which delivering would concatenate onto and submit as one message.
//
// A paste APPENDS to the box; it does not replace it. So a half-written line sitting there
// gets mangled and sent without its author ever pressing Enter. The nudge channel has
// refused a non-empty box since hq-nudge-hardening; the dispatch channel never did, which
// left `gtmux send` (CLI, HQ, and the phone) able to eat a draft silently.
//
// Two things are NOT a clobber: a box that already holds THIS delivery (our own re-send
// landing idempotently after a lost ack), and a pane with no locatable input region at all
// (a plain shell — nothing to protect, and post-submit verification judges it).
func draftBlocked(io IO, opts Opts, text string) (bool, string) {
	if opts.ClobberDraft || !opts.HasComposer {
		return false, ""
	}
	// A pane in copy/view-mode is showing its SCROLLBACK, not its composer: the box on
	// screen may be an old one scrolled into view. Reading it would refuse against text
	// nobody is typing. pasteWithGuard drops the mode a moment later (exitCopyMode), so
	// skipping here costs nothing — the delivery still happens, unguarded, which is the
	// pre-guard behavior for a case the guard cannot judge.
	if io.InMode != nil && io.InMode() {
		return false, ""
	}
	// Read the draft the way every OTHER "is there an unsubmitted draft?" caller does, and
	// for the reason region.go spells out: on a PLAIN capture the faint markers are gone,
	// so Claude Code's dim suggested-next-command GHOST text is indistinguishable from a
	// typed draft. Reading plain here refused real sends to an idle Claude pane whose box
	// was in fact empty (seen live on %7: plain read "把评论里 273 改成 265", the color read
	// correctly read nothing). DraftOfColored is identity on plain text, so a caller with
	// no color capture wired degrades rather than breaks.
	read := func() (string, bool) {
		capture := io.CaptureColor
		if capture == nil {
			capture = io.Capture
		}
		// An UNREADABLE pane fails OPEN. A capture that comes back empty means the tmux
		// call failed, the pane died, or the server is wedged — none of which is evidence
		// that someone is typing. DraftOfColored("") reports structured=false, which the
		// caller below treats as "no draft", so the send proceeds and post-submit
		// verification judges it. Fail-CLOSED here would turn any tmux hiccup into a
		// blanket refusal of every send on the machine.
		return DraftOfColored(capture())
	}
	held := func() (string, bool) {
		draft, structured := read()
		// An unlocatable input region is NOT a draft to protect: a plain shell has no
		// composer, and post-submit verification is the judge there. (The nudge channel
		// makes the opposite call because it can queue and retry; a refusal cannot.)
		if !structured || normalizeSpace(draft) == "" || draftHasDelivery(draft, text) {
			return "", false
		}
		return draft, true
	}
	if _, blocked := held(); !blocked {
		return false, ""
	}
	// TWO frames must agree, the same discipline the wake channel's guard uses: one frame
	// mid-repaint can show a phantom, and refusing a legitimate send is not free either.
	//
	// EXACTLY two reads, never a loop: the guard is a bounded pre-check on the send path,
	// so it can add a known cost (one capture, or two plus one poll interval on the way to
	// a refusal) and can never spin, hang, or retry its way into blocking a send.
	if io.Sleep != nil {
		io.Sleep()
	}
	draft, blocked := held()
	if !blocked {
		return false, ""
	}
	return true, "input box holds unsubmitted text: " + clampEvidence(draft)
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
		verdict, placed := confirmPaste(io, opts, text)
		// A fragment verdict on a pane where something DID reach the box is the one case
		// where the guard's cure is worse than the disease. The verdict authorizes C-u —
		// destroying the draft — and the scrape cannot tell "the agent rendered less than
		// we pasted" from "the agent has not finished rendering what we pasted". On an IDLE
		// Codex it is reliably the latter: the pane is still (so the busy escape above does
		// not fire) while the draft renders a beat late, and the guard wiped a perfectly
		// good paste, re-pasted into the wreckage, and the message was lost.
		//
		// So when the agent is hook-equipped, a non-empty draft is treated as PLACED and
		// submitted: the receipt then judges on the FULL needle, which a genuinely truncated
		// paste cannot match — the delivery is reported failed instead of silently mangled.
		// A hook-LESS agent has no receipt, so the scrape stays its only evidence and the
		// clear-and-retry remains (submitting a known-truncated draft would be worse).
		if verdict == pasteFragment && opts.HookEquipped && placed {
			return true
		}
		switch verdict {
		case pasteInDraft, pasteUnverifiable, pasteBusy:
			// pasteBusy: the pane is churning and the box read never confirmed, but the
			// paste is placed. Return placed (do NOT clear+retry — that would destroy a
			// good paste) and let the submit go through; the receipt / post-submit verify
			// is the authority. Withholding here is the "message never sent into a working
			// pane" bug.
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
	pasteBusy                             // the pane kept redrawing (agent busy), so the box read never confirmed — but the paste is placed
)

// confirmPaste waits for a paste to RENDER in the draft. paste-buffer returns as soon
// as tmux has written the bytes to the pty, but the agent TUI redraws on its own
// schedule and a loaded pane can be a frame behind — so the frame right after a paste
// is not proof of anything. A positive is trusted as soon as it appears (the common
// case, and it keeps a healthy send fast); "fragment" is the verdict that authorizes
// destroying and re-pasting, so it is returned only after the whole settle window has
// passed with no match.
//
// A CHURNING pane (the agent mid-turn, still redrawing — an HQ session mid-answer, or a
// Codex turn ticking its footer timer, is the recurring case) is a THIRD outcome, distinct
// from a fragment. The paste lands in the box, but the box-region scrape never catches a
// clean frame that reads back the full head+tail — the pane keeps redrawing and races every
// read. Calling that a fragment (and clearing / withholding Enter) silently drops the send
// into a working pane; the user watched their message never submit against a busy %pane. So
// the "is it still rendering?" signal watches the WHOLE capture, not just the transcript
// ABOVE the box: Codex draws its working state — an elapsed-time counter and a spinner — in
// the status footer BELOW the box, so a transcript-only diff read a working Codex as "quiet"
// and false-failed every send into it ("the input box didn't confirm the full message").
// When the settle window expires with the pane still moving AND a non-empty draft (the paste
// demonstrably reached the box), return pasteBusy — placed, submit best-effort, let the
// receipt / post-submit verify judge. A pane that goes and STAYS still is a real fragment.
func confirmPaste(io IO, opts Opts, text string) (pasteVerdict, bool) {
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
	peak := 0            // max non-empty draft length seen (the paste reached the box)
	var prevFrame string // last frame's FULL capture, to sense motion ANYWHERE on screen
	frameSeen := false   // have we captured a baseline frame to diff against
	renderStall := 0     // consecutive frames the WHOLE pane held STILL (agent not rendering)
	for i := 0; ; i++ {
		frame := io.Capture()
		_, draft, structured := SplitInputRegion(frame)
		if !structured {
			return pasteUnverifiable, peak > 0
		}
		if draftHasDelivery(draft, text) {
			return pasteInDraft, true
		}
		switch {
		case !frameSeen:
			// first frame — no baseline to diff against yet
		case frame != prevFrame:
			renderStall = 0 // something on screen moved — the agent is actively rendering
		default:
			renderStall++ // the whole pane held still this frame
		}
		prevFrame, frameSeen = frame, true
		n := len(normalizeSpace(draft))
		if n > prevLen {
			prevLen, stall = n, 0 // still arriving — do not clear a paste mid-render
		} else {
			stall++
		}
		if n > peak {
			peak = n
		}
		// The motion signal is the WHOLE capture, not just the transcript above the box, so a
		// Codex ticking its footer timer reads as busy (see the doc comment). Two asymmetric
		// thresholds decide, and they must not share one sample: a QUIET pane's frame goes
		// static and its renderStall climbs in lockstep with `stall`, so a single sample at
		// the stall point can't tell "quiet" from "mid-gap between two slow footer ticks".
		if i >= ceiling {
			// Hard cap: still moving + placed ⇒ busy (submit best-effort); else a fragment.
			if peak > 0 && renderStall < renderMotionFrames {
				return pasteBusy, true
			}
			return pasteFragment, peak > 0
		}
		if i >= budget && stall >= pasteStallFrames {
			// The draft stopped growing. Moving RIGHT NOW (renderStall short) + placed ⇒ busy
			// immediately — keeps a Claude mid-turn / just-ticked Codex send fast. Held STILL
			// across a full footer-tick window ⇒ a genuinely quiet pane, so a short draft is a
			// real fragment. In between (a gap between slow footer ticks), keep looping: the
			// next tick resets renderStall→busy, or sustained stillness→fragment.
			if renderStall < pasteStallFrames && peak > 0 {
				return pasteBusy, true
			}
			if renderStall >= renderMotionFrames {
				return pasteFragment, peak > 0
			}
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
	// renderMotionFrames: how many consecutive STILL frames prove the whole pane is quiet
	// (nothing left rendering) before an unmatched draft is called a fragment. Sized to span
	// a ~1s footer clock at the ~300ms poll cadence — Codex draws its working state (an
	// elapsed-time counter + spinner) in the status footer BELOW the input box, which ticks
	// about every 3-4 frames, so fewer still-frames than this can just be a gap between ticks,
	// not a quiet pane. Deliberately larger than pasteStallFrames: draft-growth (stall) and
	// whole-frame motion (renderStall) are different signals with different windows.
	renderMotionFrames = 6
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
	// Wrap-tolerant head+tail: a long run with no break points (a no-space CJK line)
	// is WRAPPED by the composer, and the break reads back from the capture as a space
	// that never existed in the text — so a head or tail fingerprint straddling the
	// wrap point misses (the CJK long-message "Not sent, then truncated+re-pasted"
	// bug). Retry the same head+tail match with ALL whitespace stripped from both
	// sides, which recovers a fingerprint split by a wrap (same technique the image
	// paths below already use). Space-free matching can't fabricate a match: a 40-rune
	// fingerprint reappearing verbatim is the delivery, not coincidence.
	if containsSpaceless(draft, NormalizeHead(text)) && containsSpaceless(draft, NormalizeTail(text)) {
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

// containsSpaceless reports whether haystack contains needle once ALL whitespace is
// stripped from both. It recovers a fingerprint the composer split with a wrap-induced
// space (a no-space CJK line breaks mid-run, and the break reads back as a space). An
// empty needle never matches.
func containsSpaceless(haystack, needle string) bool {
	n := stripAllSpace(needle)
	if n == "" {
		return false
	}
	return strings.Contains(stripAllSpace(haystack), n)
}

// stripAllSpace removes every whitespace rune (unlike normalizeSpace, which only
// collapses runs) so a wrap-inserted space between two runs that were adjacent in the
// text disappears.
func stripAllSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
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
// clampEvidence bounds a draft quoted back in a refusal. The draft is the USER's own
// unsubmitted text, so the refusal shows enough to recognize it ("that's my sentence")
// without echoing a whole paragraph into a log or a phone toast.
func clampEvidence(draft string) string {
	d := normalizeSpace(draft)
	const n = 60
	if len([]rune(d)) > n {
		return string([]rune(d)[:n]) + "…"
	}
	return d
}

func evidenceTail(capture string) string {
	lines := strings.Split(strings.TrimRight(capture, "\n"), "\n")
	const n = 12
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
