package dispatch

import (
	"strings"
	"testing"
)

// holdAfter answers "" for the first n calls, then why: a question that appears part way
// through a delivery. n = 0 means it is already up.
func holdAfter(n int, why string, calls *int) func() string {
	return func() string {
		*calls++
		if *calls > n {
			return why
		}
		return ""
	}
}

func TestDeliver_HoldBeforePaste_TypesNothing(t *testing.T) {
	f := &fakeIO{caps: []string{boxDraft(taskText)}}
	var calls int
	io := f.io()
	io.Hold = holdAfter(0, "a choice menu is on screen", &calls)
	r := Deliver(io, Opts{Pane: "%1", DeliverTimeout: 10}, taskText)
	if r.State != StateRefusedWaiting || r.Delivered || !strings.Contains(r.Evidence, "choice menu") {
		t.Fatalf("want refused-waiting with the reason, got %+v", r)
	}
	if f.pasteCalls != 0 || f.enterCalls != 0 || len(f.recorded) != 0 {
		t.Fatalf("nothing may be typed or recorded: paste=%d enter=%d recorded=%v", f.pasteCalls, f.enterCalls, f.recorded)
	}
}

// The question appears between the paste and the Enter: the Enter that would pick its
// default is never pressed, and the interlock is dropped so a later retry is not refused.
func TestDeliver_HoldBeforeEnter_PastesButNeverSubmits(t *testing.T) {
	f := &fakeIO{caps: []string{boxDraft(taskText), boxDraft(taskText)}}
	var calls int
	forgot := false
	io := f.io()
	io.Hold = holdAfter(1, "the agent is waiting on you", &calls)
	io.ForgetSend = func(string) { forgot = true }
	r := Deliver(io, Opts{Pane: "%1", DeliverTimeout: 10}, taskText)
	if r.State != StateRefusedWaiting || r.Delivered || !strings.HasPrefix(r.Evidence, EvidenceHeldBeforeEnter+"the agent is waiting on you") {
		t.Fatalf("want refused-waiting before Enter, got %+v", r)
	}
	if f.pasteCalls != 1 || f.enterCalls != 0 {
		t.Fatalf("pasted once, never submitted: paste=%d enter=%d", f.pasteCalls, f.enterCalls)
	}
	if !forgot {
		t.Fatal("the interlock record must be dropped for a send that was not submitted")
	}
}

// The first Enter was swallowed, and by the time a retry is due something is asking: no
// blind second Enter. The verdict says so instead of claiming a plain failure.
func TestDeliver_HoldStopsTheEnterRetry(t *testing.T) {
	f := &fakeIO{caps: []string{
		boxDraft(taskText), // paste guard
		boxDraft(taskText), // verify 1: still in the draft
		boxDraft(taskText), // verify 2: still there, a retry is due
	}}
	var calls int
	io := f.io()
	io.Hold = holdAfter(2, "a choice menu is on screen", &calls)
	r := Deliver(io, Opts{Pane: "%1", DeliverTimeout: 6, EnterRetries: 3}, taskText)
	if f.enterCalls != 1 {
		t.Fatalf("only the first Enter may be pressed; enterCalls=%d", f.enterCalls)
	}
	if r.State != StateRefusedWaiting || r.Delivered || !strings.HasPrefix(r.Evidence, EvidenceHeldBeforeRetry+"a choice menu") {
		t.Fatalf("want refused-waiting naming the stopped retry, got %+v", r)
	}
	if calls != 3 {
		t.Fatalf("Hold is asked before the paste, the Enter and the one due retry, then not again; calls=%d", calls)
	}
}

// Without a Hold, nothing changes: the swallowed-Enter retry still happens.
func TestDeliver_NoHold_IsUnchanged(t *testing.T) {
	f := &fakeIO{caps: []string{
		boxDraft(taskText), boxDraft(taskText), boxDraft(taskText),
		boxEmpty("me: " + taskText), boxEmpty("me: " + taskText),
	}}
	r := Deliver(f.io(), Opts{Pane: "%1", DeliverTimeout: 20, EnterRetries: 3}, taskText)
	if r.State != StateLanded || f.enterCalls < 2 {
		t.Fatalf("want landed with a re-Enter, got %+v enter=%d", r, f.enterCalls)
	}
}

func TestPasteAndSubmit_Hold(t *testing.T) {
	for _, tc := range []struct {
		name         string
		after        int
		paste, enter int
	}{
		{"already asking: nothing typed", 0, 0, 0},
		{"asking by the Enter: pasted, not submitted", 1, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeIO{caps: []string{boxDraft(taskText)}}
			var calls int
			io := f.io()
			io.Hold = holdAfter(tc.after, "menu", &calls)
			ok, refused := PasteAndSubmit(io, Opts{Pane: "%1"}, taskText)
			if ok || refused != StateRefusedWaiting || f.pasteCalls != tc.paste || f.enterCalls != tc.enter {
				t.Fatalf("ok=%v refused=%q paste=%d enter=%d", ok, refused, f.pasteCalls, f.enterCalls)
			}
		})
	}
}
