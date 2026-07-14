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

const taskText = "implement the verified dispatch state machine with layered checks"

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
}

func (f *fakeIO) io() IO {
	return IO{
		Capture: func() string {
			f.capCalls++
			i := f.capI
			if i >= len(f.caps) {
				i = len(f.caps) - 1
			}
			f.capI++
			return f.caps[i]
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
	if f.capCalls != 1 {
		t.Fatalf("hook path should not depend on repeated screen reads; capCalls=%d", f.capCalls)
	}
	if len(f.recorded) != 1 {
		t.Fatalf("delivery should be recorded for the interlock; got %v", f.recorded)
	}
}

func TestDeliver_Fragment_RetriesThenLands(t *testing.T) {
	f := &fakeIO{
		caps: []string{
			boxDraft("cl"),              // paste guard #1: fragment
			boxDraft(taskText),          // paste guard #2: full text (after ClearDraft+retry)
			boxEmpty("me: " + taskText), // verify frame 1: landed
			boxEmpty("me: " + taskText), // verify frame 2: landed (two-frame agree)
		},
	}
	r := Deliver(f.io(), Opts{Pane: "%1", HookEquipped: false, DeliverTimeout: 10, PasteRetries: 2}, taskText)
	if !r.Delivered || r.State != StateLanded {
		t.Fatalf("want landed after retry, got %+v", r)
	}
	if f.pasteCalls != 2 || f.clearCalls != 1 {
		t.Fatalf("fragment should ClearDraft + re-paste once; paste=%d clear=%d", f.pasteCalls, f.clearCalls)
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
