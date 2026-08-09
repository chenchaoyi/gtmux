package dispatch

import (
	"strings"
	"testing"
)

// --- frame builders (representative agent-TUI captures) ---

func boxDraft(text string) string {
	return "history line above\n" +
		"╭────────────────────────────────────────╮\n" +
		"│ ❯ " + text + " │\n" +
		"╰────────────────────────────────────────╯"
}

func boxEmpty(history string) string {
	return history + "\n" +
		"╭────────────────────────────────────────╮\n" +
		"│ ❯  │\n" +
		"╰────────────────────────────────────────╯"
}

// boxDraftLines renders a MULTI-LINE draft the way an agent TUI does: the prompt
// marker on the first line only, continuations indented under it.
func boxDraftLines(lines ...string) string {
	out := "history line above\n╭────────────────────────────────────────╮\n"
	for i, l := range lines {
		mark := "  "
		if i == 0 {
			mark = "❯ "
		}
		out += "│ " + mark + l + " │\n"
	}
	return out + "╰────────────────────────────────────────╯"
}

// busyBox renders a CHURNING pane: a transcript above the input box that keeps moving
// (histTag differs frame to frame — the agent is mid-turn, rendering), with a box whose
// draft reads back as only a PREFIX of the payload (< headRunes, so head/tail never
// confirm amid the churn). This is the "%pane mid-answer" shape where the paste landed
// but the box scrape never caught a clean frame.
func busyBox(histTag, draftPrefix string) string {
	return "streaming output " + histTag + "\n" +
		"more output " + histTag + "\n" +
		"╭────────────────────────────────────────╮\n" +
		"│ ❯ " + draftPrefix + " │\n" +
		"╰────────────────────────────────────────╯"
}

// codexBox renders a Codex-shaped capture: a STATIC transcript above the input box, the
// box drawn as two full-width horizontal rules with "❯ draft" between them, and a status
// FOOTER below it whose elapsed-time counter (footerTag) changes frame to frame while the
// agent works. The transcript never moves — the ONLY motion is in the footer — so a
// transcript-only busy heuristic reads this working pane as "quiet". The whole-frame diff
// must see the footer tick. (Real Codex 0.146.0 shape, captured live.)
func codexBox(footerTag, draftPrefix string) string {
	rule := strings.Repeat("─", 60)
	return "static transcript line one\n" +
		"static transcript line two\n" +
		rule + "\n" +
		"❯ " + draftPrefix + "\n" +
		rule + "\n" +
		"  /tmp │ Opus 5 (1M) │ Baked for " + footerTag + "   new task? /clear\n" +
		"  ⏸ manual mode on"
}

const taskText = "implement the verified dispatch state machine with layered checks"

// multiText is a multi-line instruction — the payload shape that exposed the
// duplicate-paste bug (C-u cannot clear a multi-line draft, so the old retry pasted
// on top of the leftover).
const multiText = "step one: run make check\nstep two: confirm it is green\nstep three: tag the release"

// --- fake IO ---

type fakeIO struct {
	clock                                        int64
	caps                                         []string
	capI                                         int
	evs                                          []Ev
	pasteCalls, enterCalls, clearCalls, capCalls int
	recentHash                                   string
	recentTs                                     int64
	recorded                                     []string
	inMode                                       bool // pane starts in copy/view-mode
	exitCalls                                    int  // ExitMode invocations
	exitBeforePaste                              bool // ExitMode ran before the first Paste
	// preDraft is what the input box holds BEFORE Deliver pastes anything. Deliver reads
	// it first (the draft guard: never append to someone's unsubmitted text), so the fake
	// must serve that frame or every subsequent fixture frame is off by one. Empty — an
	// idle, empty box — is the normal case and the default; a test that wants the guard to
	// fire sets it.
	preDraft string
}

// io is the seam DELIVER sees: the fixture's frames behind the PRE-PASTE frame that
// Deliver's draft guard reads before it pastes anything.
func (f *fakeIO) io() IO {
	return f.ioWith(append([]string{boxDraft(f.preDraft)}, f.caps...))
}

// rawIO serves the fixture's frames verbatim — for unit-testing confirmPaste and the
// other post-paste helpers directly, which never see Deliver's pre-paste read.
func (f *fakeIO) rawIO() IO { return f.ioWith(f.caps) }

func (f *fakeIO) ioWith(caps []string) IO {
	return IO{
		Capture: func() string {
			f.capCalls++
			i := f.capI
			if i >= len(caps) {
				i = len(caps) - 1
			}
			f.capI++
			return caps[i]
		},
		Paste:      func(string) error { f.pasteCalls++; return nil },
		Enter:      func() error { f.enterCalls++; return nil },
		ClearDraft: func() error { f.clearCalls++; return nil },
		InMode:     func() bool { return f.inMode },
		ExitMode: func() error {
			f.exitCalls++
			if f.pasteCalls == 0 {
				f.exitBeforePaste = true
			}
			f.inMode = false // cancelling copy-mode drops the pane out of it
			return nil
		},
		Events: func(since int64) []Ev {
			var out []Ev
			for _, e := range f.evs {
				if e.Ts >= since {
					out = append(out, e)
				}
			}
			return out
		},
		Now:        func() int64 { return f.clock },
		Sleep:      func() { f.clock++ },
		RecentSend: func(string) (string, int64) { return f.recentHash, f.recentTs },
		RecordSend: func(pane, hash string, ts int64) { f.recorded = append(f.recorded, pane+"|"+hash) },
	}
}

func TestDeliver_HookHappyPath_NoScreenNeeded(t *testing.T) {
	f := &fakeIO{
		caps: []string{boxDraft(taskText)}, // only the paste-guard capture
		evs:  []Ev{{Kind: EvSubmit, Head: NormalizeHead(taskText), Ts: 0}},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: 10}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("want landed, got %+v", r)
	}
	// The submit event confirmed it — only the paste-guard capture was read.
	if f.capCalls != 2 { // the draft-guard read + the one verify read
		t.Fatalf("hook path should not depend on repeated screen reads; capCalls=%d", f.capCalls)
	}
	if len(f.recorded) != 1 {
		t.Fatalf("delivery should be recorded for the interlock; got %v", f.recorded)
	}
}

func TestDeliver_Fragment_ClearedThenRePastedOnce(t *testing.T) {
	f := &fakeIO{
		caps: []string{
			// A fragment is called only once the draft stops growing AND the WHOLE pane holds
			// still through the render-motion window (renderMotionFrames) — so the same "cl"
			// must hold for that many static frames before the clear.
			boxDraft("cl"), boxDraft("cl"), boxDraft("cl"), boxDraft("cl"),
			boxDraft("cl"), boxDraft("cl"), boxDraft("cl"), // 7 static frames: a genuine fragment
			boxEmpty("history"),         // ClearDraft worked: the box is empty
			boxEmpty("history"),         // retry re-reads: nothing in the draft to keep
			boxDraft(taskText),          // paste #2: full text
			boxEmpty("me: " + taskText), // verify frame 1: landed
			boxEmpty("me: " + taskText), // verify frame 2: landed (two-frame agree)
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 10, PasteRetries: 2, PasteSettle: 1}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("want landed after retry, got %+v", r)
	}
	if f.pasteCalls != 2 || f.clearCalls != 1 {
		t.Fatalf("a CONFIRMED-clear fragment should re-paste exactly once; paste=%d clear=%d", f.pasteCalls, f.clearCalls)
	}
}

func TestConfirmPaste_CodexFooterMotion_IsBusy(t *testing.T) {
	// Codex draws its working state (an elapsed-time counter) in the status footer BELOW the
	// input box; the transcript ABOVE holds still. The old transcript-only motion diff read
	// that working pane as quiet and false-failed the send with "the input box didn't confirm
	// the full message". The whole-frame diff must see the footer tick → pasteBusy (placed,
	// submit best-effort), not pasteFragment. Footer ticks every 4 frames (a ~1s clock at the
	// poll cadence), so renderStall keeps resetting before it can reach renderMotionFrames.
	var caps []string
	for i := 0; i < 30; i++ {
		caps = append(caps, codexBox(string(rune('0'+(i/4)%10))+"s", "implement the"))
	}
	f := &fakeIO{caps: caps}
	if v, _ := confirmPaste(f.rawIO(), Opts{Pane: "%1", PasteSettle: 1}, taskText); v != pasteBusy {
		t.Fatalf("a Codex ticking its footer is busy; want pasteBusy, got %v", v)
	}
}

func TestConfirmPaste_FullyQuiet_IsFragment(t *testing.T) {
	// The other direction: a pane where NOTHING moves — same frame every read, draft only a
	// short prefix that never matches — is a genuine fragment. The whole-frame diff must still
	// reach that verdict, so the fix did not turn every unmatched draft into a best-effort
	// submit (which would resurrect the truncated-send bug on a quiet pane).
	var caps []string
	for i := 0; i < 30; i++ {
		caps = append(caps, codexBox("5s", "implement the")) // identical every frame: quiet
	}
	f := &fakeIO{caps: caps}
	if v, _ := confirmPaste(f.rawIO(), Opts{Pane: "%1", PasteSettle: 1}, taskText); v != pasteFragment {
		t.Fatalf("a fully static pane is a fragment; want pasteFragment, got %v", v)
	}
}

func TestDeliver_LateRenderedPaste_NotPastedTwice(t *testing.T) {
	// The frame right after a paste can still show the pre-paste box — the TUI redraws
	// on its own schedule. Reading that stale frame as a fragment (and pasting again)
	// is what put one instruction on screen two and three times. The paste must settle
	// first, and one paste must stay one paste.
	f := &fakeIO{
		caps: []string{
			boxEmpty("history"),         // stale frame: the paste has not rendered yet
			boxDraft(taskText),          // …and now it has
			boxEmpty("me: " + taskText), // verify frame 1: landed
			boxEmpty("me: " + taskText), // verify frame 2: landed (two-frame agree)
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 10, PasteRetries: 2, PasteSettle: 2}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("want landed, got %+v", r)
	}
	if f.pasteCalls != 1 {
		t.Fatalf("a late-rendered paste must NOT be pasted again; pasteCalls=%d", f.pasteCalls)
	}
	if f.clearCalls != 0 {
		t.Fatalf("nothing to clear — the text was in the draft; clearCalls=%d", f.clearCalls)
	}
}

func TestDeliver_UnclearableDraft_FailsRatherThanDuplicate(t *testing.T) {
	// C-u kills one line, so a multi-line draft survives it (verified on a real Claude
	// Code pane: the 2nd C-u does nothing at all). Pasting onto that leftover is what
	// concatenated a duplicate. An unclearable draft must FAIL with evidence instead —
	// one paste, no submit.
	stuck := boxDraft("step one: run make ch") // a partial paste C-u then refuses to clear
	f := &fakeIO{caps: []string{stuck}}        // every frame: the box never empties
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 10, PasteRetries: 2, PasteSettle: 1}, multiText)
	if r.Delivered || r.State != StateFailed {
		t.Fatalf("want failed, got %+v", r)
	}
	if f.pasteCalls != 1 {
		t.Fatalf("must never paste onto a draft it could not clear; pasteCalls=%d", f.pasteCalls)
	}
	if f.enterCalls != 0 {
		t.Fatalf("must not submit a fragment; enterCalls=%d", f.enterCalls)
	}
	if r.Evidence == "" {
		t.Fatalf("a refused re-paste must carry on-screen evidence")
	}
}

func TestDeliver_MultiLineText_LandsWithOneSubmit(t *testing.T) {
	// A multi-line instruction is ONE delivery: it goes in as one draft (tmux.Paste
	// brackets it) and is submitted by exactly one Enter — not one per line.
	f := &fakeIO{
		caps: []string{
			boxDraftLines("step one: run make check", "step two: confirm it is green", "step three: tag the release"),
			boxEmpty("me: " + multiText),
			boxEmpty("me: " + multiText),
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 10, PasteSettle: 1}, multiText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("want landed, got %+v", r)
	}
	if f.pasteCalls != 1 || f.enterCalls != 1 {
		t.Fatalf("one delivery = one paste + one Enter; paste=%d enter=%d", f.pasteCalls, f.enterCalls)
	}
	if r.Attempts != 1 {
		t.Fatalf("a clean landing should report a single submit attempt; got %d", r.Attempts)
	}
}

func TestDeliver_FragmentNeverCompletes_Fails(t *testing.T) {
	f := &fakeIO{caps: []string{boxDraft("cl")}} // always a fragment
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 5, PasteRetries: 2}, taskText)
	if r.Delivered || r.State != StateFailed {
		t.Fatalf("want failed, got %+v", r)
	}
	if f.enterCalls != 0 {
		t.Fatalf("must not submit a fragment; enterCalls=%d", f.enterCalls)
	}
}

func TestDeliver_BusyPaneProgressiveRender_WaitsAndLands(t *testing.T) {
	// A BUSY pane (agent mid-render) draws a long paste PROGRESSIVELY — the draft grows
	// over several frames before the full text is in the box. confirmPaste must WAIT while
	// it keeps growing (never clear a still-arriving paste) and submit once complete. This
	// is the "sending into a working pane only lands the first words, Not-sent" fix.
	// PasteSettle:1 so the OLD fixed window would have expired on frame 1 (a fragment) and
	// cleared the draft before the render finished.
	pfx := func(n int) string {
		r := []rune(taskText)
		if n > len(r) {
			n = len(r)
		}
		return string(r[:n])
	}
	f := &fakeIO{
		caps: []string{
			boxDraft(pfx(3)),            // head only, still arriving
			boxDraft(pfx(8)),            // grew
			boxDraft(pfx(15)),           // grew
			boxDraft(taskText),          // FULL — head+tail match
			boxEmpty("me: " + taskText), // landed
			boxEmpty("me: " + taskText), // landed (agree)
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 20, PasteSettle: 1, PasteRetries: 2}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("a progressively-rendering paste should land, got %+v", r)
	}
	if f.clearCalls != 0 {
		t.Fatalf("must NOT clear a still-arriving paste (that is the truncation); clearCalls=%d", f.clearCalls)
	}
	if f.pasteCalls != 1 {
		t.Fatalf("one paste, no destructive re-paste; pasteCalls=%d", f.pasteCalls)
	}
}

func TestDeliver_SwallowedEnter_ReEnters(t *testing.T) {
	f := &fakeIO{
		caps: []string{
			boxDraft(taskText),          // paste guard: full text
			boxDraft(taskText),          // verify 1: still in draft (Enter swallowed)
			boxDraft(taskText),          // verify 2: still in draft → re-Enter
			boxEmpty("me: " + taskText), // verify 3: landed
			boxEmpty("me: " + taskText), // verify 4: landed (agree)
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 20, EnterRetries: 3}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("want landed, got %+v", r)
	}
	if f.enterCalls < 2 {
		t.Fatalf("swallowed Enter should be re-sent; enterCalls=%d", f.enterCalls)
	}
}

func TestDeliver_EmptyBoxNoSubmit_NotWorking(t *testing.T) {
	// Draft emptied but the text never entered history — incident ③ "empty box + token>0".
	f := &fakeIO{
		caps: []string{
			boxDraft(taskText),          // paste guard ok
			boxEmpty("assistant: idle"), // draft empty, history WITHOUT the text
			boxEmpty("assistant: idle"),
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 4}, taskText)
	if r.Delivered || r.State != StateFailed {
		t.Fatalf("empty box with nothing submitted must be failed, got %+v", r)
	}
}

func TestDeliver_Queued_ReportedDistinctly(t *testing.T) {
	f := &fakeIO{
		caps: []string{
			boxDraft(taskText),
			boxEmpty("assistant: busy") + "\n Press up to edit queued messages",
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 10}, taskText)
	if r.Delivered || r.State != StateQueued {
		t.Fatalf("want queued, got %+v", r)
	}
}

func TestDeliver_SingleTransientFrame_NotMisjudged(t *testing.T) {
	// A single "landed-looking" frame in a delivery that never truly lands must NOT
	// yield StateLanded — the two-frame rule requires agreement (incident ⑩).
	f := &fakeIO{
		caps: []string{
			boxDraft(taskText),          // paste guard ok
			boxEmpty("me: " + taskText), // ONE transient landed-looking frame
			boxEmpty("assistant: idle"), // disagrees (no text in history)
			boxEmpty("assistant: idle"),
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 3}, taskText)
	if r.State == StateLanded {
		t.Fatalf("a single transient frame must not be trusted as landed; got %+v", r)
	}
	if r.State != StateFailed {
		t.Fatalf("want failed, got %+v", r)
	}
}

func TestDeliver_UnstructuredShell_ProceedsAndLands(t *testing.T) {
	// A plain shell pane has no locatable draft; the guard must NOT C-u (destroy) it,
	// and post-submit verify confirms the text is present (bash regression guard).
	shell := "user@host project % " + taskText
	f := &fakeIO{caps: []string{shell, shell, shell}}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 10}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("unstructured delivery should land, got %+v", r)
	}
	if f.clearCalls != 0 {
		t.Fatalf("must NOT clear-draft on an unstructured pane; clearCalls=%d", f.clearCalls)
	}
	if f.enterCalls == 0 {
		t.Fatalf("should have submitted")
	}
}

func TestDeliver_Timeout_NeverReportsSuccess(t *testing.T) {
	f := &fakeIO{caps: []string{boxDraft(taskText), boxDraft(taskText)}}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, HookGrace: 100, DeliverTimeout: 3}, taskText)
	// hook-equipped but no submit event ever arrives, grace never elapses → timeout.
	if r.Delivered || r.State != StateFailed {
		t.Fatalf("timeout must be failed, got %+v", r)
	}
	if r.Evidence == "" {
		t.Fatalf("a failure must carry on-screen evidence")
	}
}

func TestDeliver_Interlock_RefusesDuplicate(t *testing.T) {
	dup := PayloadHash("%1", taskText)
	f := &fakeIO{
		caps:       []string{boxDraft(taskText)},
		recentHash: dup,
		recentTs:   100,
		clock:      110, // within a 60s window
	}
	r := Deliver(f.io(), Opts{Pane: "%1", ResendWindow: 60, DeliverTimeout: 5}, taskText)
	if r.State != StateRefusedDup {
		t.Fatalf("want refused-duplicate, got %+v", r)
	}
	if f.pasteCalls != 0 || f.enterCalls != 0 {
		t.Fatalf("a refused duplicate must deliver nothing; paste=%d enter=%d", f.pasteCalls, f.enterCalls)
	}
}

func TestDeliver_Interlock_ForceOverrides(t *testing.T) {
	dup := PayloadHash("%1", taskText)
	f := &fakeIO{
		caps:       []string{boxDraft(taskText), boxEmpty("me: " + taskText), boxEmpty("me: " + taskText)},
		recentHash: dup,
		recentTs:   100,
		clock:      110,
	}
	r := Deliver(f.io(), Opts{Pane: "%1", Force: true, ResendWindow: 60, DeliverTimeout: 10}, taskText)
	if !r.Delivered {
		t.Fatalf("--force must override the interlock, got %+v", r)
	}
	if f.pasteCalls == 0 {
		t.Fatalf("forced delivery should paste")
	}
}

func TestDeliver_CopyMode_ExitedBeforePaste(t *testing.T) {
	// A pane scrolled into copy/view-mode eats paste-buffer + Enter as mode-nav, so a
	// delivery silently vanishes. Deliver must ExitMode BEFORE the first Paste, then
	// the payload lands normally.
	f := &fakeIO{
		inMode: true,
		caps: []string{
			boxDraft(taskText),          // paste guard: full text (mode already exited)
			boxEmpty("me: " + taskText), // verify 1: landed
			boxEmpty("me: " + taskText), // verify 2: landed (two-frame agree)
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 10}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("want landed after exiting copy-mode, got %+v", r)
	}
	if f.exitCalls == 0 || !f.exitBeforePaste {
		t.Fatalf("copy-mode must be exited before pasting; exitCalls=%d before=%v", f.exitCalls, f.exitBeforePaste)
	}
	if f.pasteCalls == 0 {
		t.Fatalf("delivery should still paste after exiting the mode")
	}
}

func TestDeliver_NotInMode_NoCancel(t *testing.T) {
	// The common case: pane not in a mode → no spurious `-X cancel` (which would error
	// "not in a mode" and could disturb a non-scrolled pane).
	f := &fakeIO{
		caps: []string{
			boxDraft(taskText),
			boxEmpty("me: " + taskText),
			boxEmpty("me: " + taskText),
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 10}, taskText)
	if !r.Delivered {
		t.Fatalf("want landed, got %+v", r)
	}
	if f.exitCalls != 0 {
		t.Fatalf("must not cancel a pane that is not in a mode; exitCalls=%d", f.exitCalls)
	}
}

func TestDeliver_HeadOnlyDraft_NotSubmittedAsWhole(t *testing.T) {
	// The truncation bug: a long payload whose HEAD renders but whose TAIL never
	// arrives. The old confirm matched on the 40-rune head alone and fired Enter,
	// submitting a truncated draft that the fallback then misread as landed. The
	// head-only draft must be treated as a fragment — never submitted.
	headOnly := boxDraft(NormalizeHead(taskText)) // only the leading fingerprint is in the box
	f := &fakeIO{caps: []string{headOnly}}        // the tail never comes
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 5, PasteRetries: 0, PasteSettle: 1}, taskText)
	if r.Delivered || r.State != StateFailed {
		t.Fatalf("a head-only draft must not be a success, got %+v", r)
	}
	if f.enterCalls != 0 {
		t.Fatalf("must not submit a head-only (truncated) draft; enterCalls=%d", f.enterCalls)
	}
}

func TestDeliver_TailRendersLate_WaitedNotTruncated(t *testing.T) {
	// The head renders a frame before the tail. The confirm must WAIT for the tail
	// within the settle window and submit the ONE complete paste — not fire Enter on
	// the head and cut the task off.
	f := &fakeIO{
		caps: []string{
			boxDraft(NormalizeHead(taskText)), // confirm i=0: only the head so far
			boxDraft(taskText),                // confirm i=1: head AND tail present
			boxEmpty("me: " + taskText),       // verify 1: landed
			boxEmpty("me: " + taskText),       // verify 2: landed (two-frame agree)
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 10, PasteSettle: 2}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("want landed once the tail arrives, got %+v", r)
	}
	if f.pasteCalls != 1 {
		t.Fatalf("the late tail must be waited out, not re-pasted; pasteCalls=%d", f.pasteCalls)
	}
	if f.enterCalls != 1 {
		t.Fatalf("exactly one submit of the whole payload; enterCalls=%d", f.enterCalls)
	}
}

func TestDeliver_NoBlindReEnter_OnceDraftEmpty(t *testing.T) {
	// "一致才发 Enter,别盲补回车": after the first Enter, if the draft is empty (or no
	// longer holds the full target) the swallowed-Enter retry must NOT fire again.
	f := &fakeIO{
		caps: []string{
			boxDraft(taskText),          // paste guard: full text
			boxEmpty("assistant: idle"), // draft empty, text NOT in history (a mismatch)
			boxEmpty("assistant: idle"),
			boxEmpty("assistant: idle"),
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 4, EnterRetries: 3}, taskText)
	if r.Delivered {
		t.Fatalf("nothing landed, must not report success; got %+v", r)
	}
	if f.enterCalls != 1 {
		t.Fatalf("Enter must fire once and not be blindly re-sent against an empty draft; enterCalls=%d", f.enterCalls)
	}
}

func TestPasteAndSubmit_ConfirmsFullDraftThenEnters(t *testing.T) {
	// The unverified send core: wait for the FULL payload in the draft, THEN one Enter.
	f := &fakeIO{
		caps: []string{
			boxDraft(NormalizeHead(taskText)), // confirm i=0: head only
			boxDraft(taskText),                // confirm i=1: full
		},
	}
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteSettle: 2}, taskText)
	if !ok {
		t.Fatalf("a full paste must confirm")
	}
	if f.pasteCalls != 1 || f.enterCalls != 1 {
		t.Fatalf("one paste + one Enter after confirming; paste=%d enter=%d", f.pasteCalls, f.enterCalls)
	}
}

func TestPasteAndSubmit_WrappedCJKLine_ConfirmsNotChurns(t *testing.T) {
	// The CJK long-message truncation: a long run with no break points (a Chinese line
	// has no spaces) WRAPS in the composer, and the wrap reads back from the capture as
	// a space that never existed in the text — so the tail fingerprint, which straddles
	// the wrap point, misses. Plain head+tail can therefore NEVER confirm, and the guard
	// used to clear the (perfectly good) draft and mangle a re-paste, submitting a
	// truncated + duplicated mess after reporting "Not sent". The wrap-tolerant,
	// whitespace-free match recognizes the full delivery and submits it once.
	cjk := "这是一个用来复现中文长句截断问题的测试消息请不要执行任何操作只需要忽略即可谢谢配合我们正在调试发送确认逻辑务必保持原样"
	rs := []rune(cjk)
	wrapped := boxDraftLines(string(rs[:43]), string(rs[43:])) // composer wrapped it mid-run

	// Precondition: this is exactly the draft plain head+tail cannot confirm.
	if ContainsHead(wrapped, cjk) && ContainsTail(wrapped, cjk) {
		t.Fatal("precondition: plain head+tail should MISS across the wrap-inserted space")
	}

	f := &fakeIO{caps: []string{wrapped}}
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteSettle: 1}, cjk)
	if !ok {
		t.Fatal("a wrapped CJK line holds the FULL delivery — must confirm, not churn")
	}
	if f.enterCalls != 1 {
		t.Fatalf("must submit exactly once; enterCalls=%d", f.enterCalls)
	}
	if f.clearCalls != 0 {
		t.Fatalf("must NOT clear a good draft (that is what mangled the re-paste); clearCalls=%d", f.clearCalls)
	}
}

func TestPasteAndSubmit_WithholdsEnterOnFragment(t *testing.T) {
	// A settled fragment the guard cannot place must NOT be submitted — that is the
	// truncated submit this fixes. Enter is withheld.
	f := &fakeIO{caps: []string{boxDraft("cl")}} // always a fragment
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteRetries: 0, PasteSettle: 1}, taskText)
	if ok {
		t.Fatalf("a fragment must not report confirmed")
	}
	if f.enterCalls != 0 {
		t.Fatalf("must not submit a fragment; enterCalls=%d", f.enterCalls)
	}
}

func TestConfirmPaste_BusyPaneReturnsPasteBusy(t *testing.T) {
	// A CHURNING transcript (histTag differs every frame — the agent is mid-turn) with a
	// placed-but-unconfirmable draft is the busy-pane verdict, NOT a fragment.
	f := &fakeIO{caps: []string{
		busyBox("t0", "implement the"),
		busyBox("t1", "implement the"),
		busyBox("t2", "implement the"),
		busyBox("t3", "implement the"),
	}}
	if v, _ := confirmPaste(f.rawIO(), Opts{PasteSettle: 1}, taskText); v != pasteBusy {
		t.Fatalf("churning pane with a placed paste must be pasteBusy, got %v", v)
	}
}

func TestConfirmPaste_QuietFragmentStaysFragment(t *testing.T) {
	// The SAME short draft on a QUIET pane (static transcript, no churn) is still a real
	// fragment — the busy carve-out must not swallow the truncation guard.
	f := &fakeIO{caps: []string{boxDraft("cl")}} // fixed history → churn stays 0
	if v, _ := confirmPaste(f.rawIO(), Opts{PasteSettle: 1}, taskText); v != pasteFragment {
		t.Fatalf("quiet short draft must stay pasteFragment, got %v", v)
	}
}

func TestConfirmPaste_MultilineGrewThenFragment_StaysFragment(t *testing.T) {
	// A multi-line paste GROWS the input box: its top border eats a transcript line each
	// frame, so the history region changes for a frame or two before everything settles.
	// That early growth-churn must NOT be read as a busy (mid-turn) pane — once the box
	// stops growing the transcript holds still and a still-short draft is a real fragment.
	// (The busy signal keys on RECENT transcript motion, not cumulative, exactly so this
	// case is not mistaken for a churning pane.)
	grew0 := "h1\nh2\nh3\n╭──────────────╮\n│ ❯ implement the │\n╰──────────────╯"
	grew1 := "h1\nh2\n╭──────────────╮\n│ ❯ implement the │\n│  verified disp │\n╰──────────────╯"
	f := &fakeIO{caps: []string{grew0, grew1, grew1, grew1, grew1}}
	if v, _ := confirmPaste(f.rawIO(), Opts{PasteSettle: 1}, taskText); v != pasteFragment {
		t.Fatalf("a multi-line paste that grew then settled short is a fragment, got %v", v)
	}
}

func TestPasteAndSubmit_BusyPaneSubmitsBestEffort(t *testing.T) {
	// The %pane-mid-turn bug: a churning pane never yields a clean frame where the box
	// read confirms the full head+tail, but the paste DID land. With the real PasteRetries=2
	// the old guard CLEARED the draft (C-u kills a line of the user's good paste) and then
	// reported failure — so the send both failed AND mangled the composer against a working
	// pane. It must now submit best-effort with NO destructive clear.
	f := &fakeIO{caps: []string{
		busyBox("t0", "implement the"),
		busyBox("t1", "implement the"),
		busyBox("t2", "implement the"),
		busyBox("t3", "implement the"),
		busyBox("t4", "implement the"),
	}}
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteRetries: 2, PasteSettle: 1}, taskText)
	if !ok {
		t.Fatal("a busy pane holding the paste must submit best-effort, not withhold")
	}
	if f.enterCalls != 1 {
		t.Fatalf("must submit exactly once; enterCalls=%d", f.enterCalls)
	}
	if f.clearCalls != 0 {
		t.Fatalf("must NOT clear a good paste on a busy pane (C-u mangles it); clearCalls=%d", f.clearCalls)
	}
	if f.pasteCalls != 1 {
		t.Fatalf("must NOT re-paste (that would duplicate); pasteCalls=%d", f.pasteCalls)
	}
}

func TestDeliver_BusyHookPane_SubmitsThenReceiptLands(t *testing.T) {
	// End-to-end: the same churn that stalls the phone send also hit verified dispatch
	// (HQ's own dispatches go through Deliver). It must submit and let the UserPromptSubmit
	// receipt judge — landing, not failing with the paste stuck in the box and the draft
	// cleared by the retry.
	f := &fakeIO{
		caps: []string{
			busyBox("t0", "implement the"),
			busyBox("t1", "implement the"),
			busyBox("t2", "implement the"),
			busyBox("t3", "implement the"),
		},
		evs: []Ev{{Kind: EvSubmit, Head: NormalizeHead(taskText), Ts: 0}},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, PasteRetries: 2, PasteSettle: 1, DeliverTimeout: 30}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("busy hook pane: want landed via receipt, got %+v", r)
	}
	if f.clearCalls != 0 {
		t.Fatalf("must NOT clear a good paste on a busy pane; clearCalls=%d", f.clearCalls)
	}
	if f.enterCalls < 1 {
		t.Fatalf("must submit; enterCalls=%d", f.enterCalls)
	}
}

func TestPasteAndSubmit_UnstructuredShellSubmits(t *testing.T) {
	// A plain shell has no locatable draft; the command must still submit (best-effort).
	shell := "user@host project % " + taskText
	f := &fakeIO{caps: []string{shell}}
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteSettle: 1}, taskText)
	if !ok {
		t.Fatalf("an unstructured pane should proceed to submit")
	}
	if f.enterCalls != 1 {
		t.Fatalf("should submit once; enterCalls=%d", f.enterCalls)
	}
	if f.clearCalls != 0 {
		t.Fatalf("must not clear-draft on an unstructured pane; clearCalls=%d", f.clearCalls)
	}
}

func TestEvidenceTail(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, "line")
	}
	got := evidenceTail(strings.Join(lines, "\n"))
	if strings.Count(got, "line") != 12 {
		t.Fatalf("evidence should keep the last 12 lines, got %d", strings.Count(got, "line"))
	}
}

// --- P1: receipt arbitration (openspec agent-drivers) ---

// The REAL misjudgment timeline, replayed (task 1.6): the payload carries a prefix
// the hook-side cleaning strips (here a gtmux wake line), so under the old
// dual-track normalization the event head and the verifier's needle never matched —
// the genuine UserPromptSubmit sat in the stream while the screen fallback (history
// scrolled away) ran out the clock and reported NOT delivered. With the single
// NormalizeNeedle pipeline the event confirms it.
func TestDeliver_StrippablePrefix_EventStillMatches(t *testing.T) {
	payload := "» gtmux·done  gtmux:0.0 (%14) │ goal:\"x\"\n继续 P2，按 tasks.md 逐项落地"
	f := &fakeIO{
		caps: []string{boxDraftLines("» gtmux·done  gtmux:0.0 (%14) │ goal:\"x\"", "继续 P2，按 tasks.md 逐项落地")},
		// The event Summary is what the hook records: the needle pipeline's output.
		evs: []Ev{{Kind: EvSubmit, Head: NormalizeNeedle(payload), Ts: 0}},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: 10}, payload)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("the stream proved the landing; got %+v", r)
	}
	if r.JudgedBy != JudgedByDriver {
		t.Fatalf("JudgedBy = %q, want driver", r.JudgedBy)
	}
}

// A submit event that arrives BETWEEN the last poll and the deadline must not be
// lost to the timeout: the final receipt re-check reads the stream once more before
// any failure verdict (invariant I2 — screen evidence cannot outrank a
// stream-confirmed landing).
func TestDeliver_TimeoutRecheck_LateEventNotLost(t *testing.T) {
	const deadline = 6
	f := &fakeIO{caps: []string{
		boxDraft(taskText),           // paste guard: draft holds the payload
		boxEmpty("scrolled history"), // then the text is nowhere on screen
	}}
	io := f.io()
	// Screen reads cost time (the real 300ms cadence): each capture advances the
	// clock, so the deadline lands between an event scan and the deadline check.
	baseCap := io.Capture
	io.Capture = func() string { f.clock += 2; return baseCap() }
	// The submit event only becomes visible at/after the deadline moment.
	io.Events = func(since int64) []Ev {
		if f.clock < deadline {
			return nil
		}
		return []Ev{{Kind: EvSubmit, Head: NormalizeNeedle(taskText), Ts: deadline}}
	}
	r := Deliver(io, Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: deadline}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("late event must be caught by the final re-check, got %+v", r)
	}
	if r.JudgedBy != JudgedByDriver {
		t.Fatalf("JudgedBy = %q, want driver", r.JudgedBy)
	}
}

// Attribution: a screen-confirmed landing and a screen-judged failure both say so.
func TestDeliver_JudgedByScreen_Attributed(t *testing.T) {
	// Hook-less two-frame landing → screen.
	f := &fakeIO{caps: []string{
		boxDraft(taskText),
		boxEmpty("me: " + taskText),
		boxEmpty("me: " + taskText),
	}}
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 10}, taskText)
	if !r.Delivered || r.JudgedBy != JudgedByScreen {
		t.Fatalf("screen landing must be attributed to screen, got %+v", r)
	}
	// Hook-equipped timeout with a silent stream → the failure is the screen's call.
	f2 := &fakeIO{caps: []string{boxDraft(taskText), boxDraft(taskText)}}
	r2 := Deliver(f2.io(), Opts{Pane: "%1", HookEquipped: true, HookGrace: 100, DeliverTimeout: 3}, taskText)
	if r2.Delivered || r2.JudgedBy != JudgedByScreen {
		t.Fatalf("timeout failure must be attributed to screen, got %+v", r2)
	}
}

// --- attachment-chip verification (the phone's "uploaded image never sends" bug) ---
//
// Claude Code folds a pasted image path into an "[Image #N]" chip the moment it
// lands, so the path text is never in the draft verbatim. The guard used to settle
// on that as a fragment and withhold Enter — the photo sat placed-but-unsubmitted
// while the phone showed a send failure.

func TestPasteAndSubmit_ImagePathFoldedIntoChip_Submits(t *testing.T) {
	path := "/Users/x/.local/share/gtmux/uploads/ef153468-markup.png"
	f := &fakeIO{caps: []string{boxDraft("[Image #1]")}}
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteSettle: 2}, path)
	if !ok || f.enterCalls != 1 {
		t.Fatalf("chip in draft = delivery placed; want submit, got ok=%v enters=%d", ok, f.enterCalls)
	}
	if f.pasteCalls != 1 {
		t.Fatalf("no re-paste on a placed delivery; pastes=%d", f.pasteCalls)
	}
}

func TestPasteAndSubmit_BodyPlusImagePath_ChipAndProseSubmit(t *testing.T) {
	text := "看看这个报错截图\n/Users/x/.local/share/gtmux/uploads/a1b2c3d4-shot.jpg"
	f := &fakeIO{caps: []string{boxDraftLines("看看这个报错截图 [Image #1]")}}
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteSettle: 2}, text)
	if !ok || f.enterCalls != 1 {
		t.Fatalf("prose matched + chip shown; want submit, got ok=%v enters=%d", ok, f.enterCalls)
	}
}

func TestPasteAndSubmit_ImagePathButNoChipNoPath_StillWithheld(t *testing.T) {
	path := "/Users/x/.local/share/gtmux/uploads/ef153468-markup.png"
	f := &fakeIO{caps: []string{boxEmpty("history")}} // box stays empty: nothing landed
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteRetries: 0, PasteSettle: 1}, path)
	if ok || f.enterCalls != 0 {
		t.Fatalf("no chip and no path = not placed; want withheld, got ok=%v enters=%d", ok, f.enterCalls)
	}
}

func TestPasteAndSubmit_ProseWithStaleChip_NotMistakenForDelivery(t *testing.T) {
	// A draft holding an old attachment chip must not vouch for a PROSE delivery
	// that never landed — the chip path only applies when the text had image paths.
	f := &fakeIO{caps: []string{boxDraft("[Image #1]")}}
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteRetries: 0, PasteSettle: 1}, taskText)
	if ok || f.enterCalls != 0 {
		t.Fatalf("chip must not confirm unrelated prose; got ok=%v enters=%d", ok, f.enterCalls)
	}
}

func TestStripImagePathLines(t *testing.T) {
	rest, imgs := stripImagePathLines("body line\n/a/b/c-photo.HEIC\n/a/b/d.png")
	if len(imgs) != 2 || rest != "body line" {
		t.Fatalf("paths stripped, prose kept; got imgs=%v rest=%q", imgs, rest)
	}
	rest, imgs = stripImagePathLines("no attachments here")
	if len(imgs) != 0 || rest != "no attachments here" {
		t.Fatalf("prose untouched; got imgs=%v rest=%q", imgs, rest)
	}
	if _, imgs = stripImagePathLines("look at /a/b.png please"); len(imgs) != 0 {
		t.Fatalf("a prose line mentioning a path is not a bare path line")
	}
}

func TestPasteAndSubmit_LiteralWrappedImagePath_Submits(t *testing.T) {
	// The pane kept the pasted image path LITERAL (no [Image #N] chip) and the long path
	// WRAPPED in the input box — so the draft shows a space at the wrap the path never
	// had, and the whole-text tail check misses. It must still submit: prose present +
	// the path present literally (matched whitespace-free). This is the "text + image,
	// everything is in the box but Not-sent" report.
	text := "paired devices info is thin, no iOS/browser version?\n" +
		"/Users/x/.local/share/gtmux/uploads/f163a4fc-markup.png"
	// draft: prose, then the path wrapped after "f163a4fc-" (a newline the capture renders
	// mid-token) — normalizeSpace would turn that wrap into a space.
	draft := boxDraftLines(
		"paired devices info is thin, no iOS/browser version?",
		"/Users/x/.local/share/gtmux/uploads/f163a4fc-",
		"markup.png",
	)
	f := &fakeIO{caps: []string{draft}}
	ok, _ := PasteAndSubmit(f.io(), Opts{Pane: "%1", PasteSettle: 2}, text)
	if !ok || f.enterCalls != 1 {
		t.Fatalf("literal wrapped path + prose = placed; want submit, got ok=%v enters=%d", ok, f.enterCalls)
	}
	if f.pasteCalls != 1 {
		t.Fatalf("no destructive re-paste on a placed delivery; pastes=%d", f.pasteCalls)
	}
}

func TestDraftHoldsImagePaths_TruncatedPathIsNotHeld(t *testing.T) {
	// A path only PARTLY in the draft (the tail never arrived) must NOT count as placed —
	// the wrap-tolerant match must not paper over a genuine fragment.
	full := []string{"/Users/x/.local/share/gtmux/uploads/abcd1234-markup.png"}
	if draftHoldsImagePaths("❯ /Users/x/.local/share/gtmux/uploads/abcd1234-", full) {
		t.Fatalf("a truncated path must not be reported as held")
	}
	if !draftHoldsImagePaths("❯ /Users/x/.local/share/gtmux/uploads/abcd1234-\nmarkup.png", full) {
		t.Fatalf("a wrapped-but-complete path must be reported as held")
	}
}

// B (send-idempotent-receipt): a HOOK-EQUIPPED agent whose pre-submit draft scrape never
// confirms the full paste (a persistent "fragment" verdict — the redraw race behind the
// recurring false "input box didn't confirm") must NOT abort. It submits and defers to the
// UserPromptSubmit receipt, which confirms the full delivery landed.
func TestDeliver_HookEquipped_FragmentDefersToReceipt(t *testing.T) {
	f := &fakeIO{
		caps: []string{
			boxDraft("frag"), boxDraft("frag"), boxDraft("frag"), boxDraft("frag"), // stalled fragment
		},
		evs: []Ev{{Kind: EvSubmit, Head: NormalizeHead(taskText), Ts: 0}},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: 10, PasteRetries: 0, PasteSettle: 1}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("hook-equipped: a fragment verdict must defer to the receipt (landed), got %+v", r)
	}
	if r.JudgedBy != JudgedByDriver {
		t.Errorf("landing should be judged by the receipt, got %q", r.JudgedBy)
	}
	if len(f.recorded) != 1 {
		t.Errorf("the send should still be recorded for the interlock; got %v", f.recorded)
	}
}

// A hook-LESS agent has no receipt, so a settled fragment must still FAIL (submitting a
// known-truncated draft is worse than reporting it). B changes only the hook-equipped path.
func TestDeliver_HookLess_FragmentStillFails(t *testing.T) {
	f := &fakeIO{
		caps: []string{boxDraft("frag"), boxDraft("frag"), boxDraft("frag"), boxDraft("frag")},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 10, PasteRetries: 0, PasteSettle: 1}, taskText)
	if r.Delivered || r.State != StateFailed {
		t.Fatalf("hook-less: a settled fragment must still fail (no receipt to trust), got %+v", r)
	}
}

// ── the three delivery-safety rules (send-delivery-safety) ───────────────────

// A paste APPENDS to the input box. If the user has a half-written line sitting there,
// delivering concatenates the payload onto it and submits both as one message — their
// sentence mangled and sent without them ever pressing Enter. The nudge channel has
// refused a non-empty box since hq-nudge-hardening; the dispatch channel never did.
func TestDeliver_RefusesToClobberAUsersDraft(t *testing.T) {
	f := &fakeIO{
		preDraft: "notes I was still writing",
		caps:     []string{boxEmpty("me: " + taskText)},
		evs:      []Ev{{Kind: EvSubmit, Head: NormalizeHead(taskText), Ts: 0}},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: 10}, taskText)
	if r.State != StateRefusedDraft {
		t.Fatalf("want refused-draft against a user's unsubmitted text, got %+v", r)
	}
	if r.Delivered {
		t.Error("a refusal is not a delivery")
	}
	// Nothing may touch the pane: not the paste, not Enter, and above all not C-u.
	if f.pasteCalls != 0 || f.enterCalls != 0 || f.clearCalls != 0 {
		t.Errorf("the refusal must not write to the pane; paste=%d enter=%d clear=%d",
			f.pasteCalls, f.enterCalls, f.clearCalls)
	}
	// The caller has to be able to recognize whose text it was.
	if !strings.Contains(r.Evidence, "notes I was still writing") {
		t.Errorf("the refusal must quote the draft it protected, got %q", r.Evidence)
	}
}

// The operator's explicit override delivers through the guard.
func TestDeliver_ClobberDraftOverridesTheGuard(t *testing.T) {
	f := &fakeIO{
		preDraft: "half a sentence",
		caps:     []string{boxEmpty("me: " + taskText)},
		evs:      []Ev{{Kind: EvSubmit, Head: NormalizeHead(taskText), Ts: 0}},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, ClobberDraft: true, DeliverTimeout: 10}, taskText)
	if !r.Delivered {
		t.Fatalf("ClobberDraft must deliver through the draft guard, got %+v", r)
	}
}

// …but Force ALONE must not, and this is the property that protects the phone. serve sets
// Force on every `/api/send` (it carries its own sendID idempotency), so folding the two
// overrides into one would leave the surface most likely to clobber a draft — a send from
// another device into a pane whose owner is typing — the one surface with no protection.
func TestDeliver_InterlockForceDoesNotWaiveTheDraftGuard(t *testing.T) {
	f := &fakeIO{
		preDraft: "half a sentence",
		caps:     []string{boxEmpty("me: " + taskText)},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, Force: true, DeliverTimeout: 10}, taskText)
	if r.State != StateRefusedDraft {
		t.Fatalf("interlock --force must not waive draft protection, got %+v", r)
	}
}

// The UNVERIFIED fast path (what POST /api/send uses) refuses too, and says which refusal
// it was so the phone can tell the user something true.
func TestPasteAndSubmit_RefusesADraft(t *testing.T) {
	f := &fakeIO{preDraft: "mid-sentence", caps: []string{boxEmpty("")}}
	ok, refused := PasteAndSubmit(f.io(), Opts{Pane: "%1"}, taskText)
	if ok || refused != StateRefusedDraft {
		t.Fatalf("want a draft refusal, got ok=%v refused=%q", ok, refused)
	}
	if f.pasteCalls != 0 || f.enterCalls != 0 {
		t.Errorf("the refusal must not write to the pane; paste=%d enter=%d", f.pasteCalls, f.enterCalls)
	}
}

// Our OWN text in the box is not someone else's draft: a re-send whose paste already
// landed (the ack was lost, not the message) must proceed and confirm idempotently,
// exactly as it did before the guard existed.
func TestDeliver_OwnDraftIsNotAClobber(t *testing.T) {
	f := &fakeIO{
		preDraft: taskText,
		caps:     []string{boxEmpty("me: " + taskText)},
		evs:      []Ev{{Kind: EvSubmit, Head: NormalizeHead(taskText), Ts: 0}},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: 10}, taskText)
	if r.State == StateRefusedDraft {
		t.Fatalf("a draft holding OUR delivery must not be treated as a clobber, got %+v", r)
	}
}

// The interlock records the payload when the paste is PLACED — right, because a crash
// between paste and submit must not double-deliver. But it was read as "this was
// delivered", so a send that FAILED verification refused its own retry: `gtmux send`
// answered refused-duplicate to the one thing the operator obviously wanted. Against a
// Codex whose receipt channel was dead, that was every retry.
func TestDeliver_FailedSendDoesNotPoisonItsRetry(t *testing.T) {
	forgotten := ""
	f := &fakeIO{caps: []string{boxDraft(taskText), boxDraft(taskText)}} // never lands
	io := f.io()
	io.ForgetSend = func(pane string) { forgotten = pane }
	r := Deliver(io, Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: 1, HookGrace: 0}, taskText)
	if r.State != StateFailed {
		t.Fatalf("setup: want a failed delivery, got %+v", r)
	}
	if forgotten != "%1" {
		t.Errorf("a failed delivery must drop its interlock record; forgot %q", forgotten)
	}
}

// …but an ACCEPTED delivery keeps its record. StateQueued means the agent took the
// message and is running it behind the current turn; re-sending would duplicate the
// instruction, which is the whole reason the interlock exists.
func TestDeliver_QueuedSendKeepsItsInterlock(t *testing.T) {
	forgotten := ""
	f := &fakeIO{caps: []string{
		boxDraft(taskText),
		boxEmpty("assistant: busy") + "\n Press up to edit queued messages",
	}}
	io := f.io()
	io.ForgetSend = func(pane string) { forgotten = pane }
	r := Deliver(io, Opts{Pane: "%1", DeliverTimeout: 10}, taskText)
	if r.State != StateQueued {
		t.Fatalf("setup: want queued, got %+v", r)
	}
	if forgotten != "" {
		t.Errorf("an accepted delivery must keep its interlock record; forgot %q", forgotten)
	}
}

// The fragment verdict authorizes C-u — destroying the draft — and the scrape cannot tell
// "the agent rendered less than we pasted" from "the agent has not finished rendering what
// we pasted". On an IDLE Codex it is reliably the latter: the pane is still (so the busy
// escape does not fire) while the draft renders a beat late. The guard wiped a good paste
// and the message was lost. With a receipt to judge on the FULL needle, a placed draft is
// submitted instead of destroyed.
func TestDeliver_IdlePaneFragmentIsNotDestroyed(t *testing.T) {
	f := &fakeIO{
		// A still pane whose draft reads short every frame — the slow-render race.
		caps: []string{
			boxDraft("implement the"), boxDraft("implement the"), boxDraft("implement the"),
			boxDraft("implement the"), boxEmpty("me: " + taskText),
		},
		evs: []Ev{{Kind: EvSubmit, Head: NormalizeHead(taskText), Ts: 0}},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: 10,
		PasteRetries: 2, PasteSettle: 1}, taskText)
	if f.clearCalls != 0 {
		t.Errorf("a hook-equipped pane's placed draft must never be cleared; clearCalls=%d", f.clearCalls)
	}
	if f.pasteCalls != 1 {
		t.Errorf("…and must not be re-pasted on top of itself; pasteCalls=%d", f.pasteCalls)
	}
	if !r.Delivered || r.JudgedBy != JudgedByDriver {
		t.Errorf("the receipt should judge the delivery, got %+v", r)
	}
}

// The hook-LESS half is unchanged: with no receipt the scrape is the only evidence there
// is, so a settled fragment must still clear and retry rather than submit known-truncated
// text. This is the boundary of the rule above.
func TestDeliver_HookLessFragmentStillClearsAndRetries(t *testing.T) {
	f := &fakeIO{caps: []string{
		boxDraft("frag"), boxDraft("frag"), boxDraft("frag"), boxDraft("frag"),
		boxDraft("frag"), boxDraft("frag"), boxDraft("frag"), boxDraft("frag"),
	}}
	Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 2,
		PasteRetries: 1, PasteSettle: 1}, taskText)
	if f.clearCalls == 0 {
		t.Error("a hook-less agent has no receipt — the fragment retry must still clear")
	}
}
