package dispatch

import (
	"errors"
	"strings"
	"testing"
)

func TestSPL008SubmissionBoundary(t *testing.T) {
	transportErr := errors.New("synthetic transport detail")
	for _, tc := range []struct {
		name               string
		pasteErr, enterErr error
		holdAfterPaste     bool
		wantCalls          int
		wantCode           InputFailureCode
		wantBoundary       Submission
		wantState          State
	}{
		{name: "capacity", pasteErr: &InputError{Code: InputSpoolFull, Submission: SubmissionNotSubmitted, Cause: transportErr},
			wantCode: InputSpoolFull, wantBoundary: SubmissionNotSubmitted, wantState: StateFailed},
		{name: "prior unknown", pasteErr: &InputError{Code: InputOutcomeUnconfirmed, Submission: SubmissionUnknown, Cause: transportErr},
			wantCode: InputOutcomeUnconfirmed, wantBoundary: SubmissionUnknown, wantState: StateFailed},
		{name: "settled fragment", pasteErr: errUnconfirmedPaste,
			wantCode: InputUnconfirmed, wantBoundary: SubmissionNotSubmitted, wantState: StateFailed},
		{name: "paste then hold", holdAfterPaste: true,
			wantCode: InputUnavailable, wantBoundary: SubmissionNotSubmitted, wantState: StateRefusedWaiting},
		{name: "pre-Enter rejection", enterErr: &InputError{Code: InputTargetChanged, Submission: SubmissionNotSubmitted, Cause: transportErr},
			wantCalls: 1, wantCode: InputTargetChanged, wantBoundary: SubmissionNotSubmitted, wantState: StateFailed},
		{name: "Enter outcome unknown", enterErr: transportErr,
			wantCalls: 1, wantCode: InputOutcomeUnconfirmed, wantBoundary: SubmissionUnknown, wantState: StateFailed},
		{name: "submitted", wantCalls: 1, wantState: StateSubmitted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pastes, enters := 0, 0
			io := IO{
				Capture: func() string { return "" }, Sleep: func() {},
				Paste: func(string) error { pastes++; return tc.pasteErr },
				Enter: func() error { enters++; return tc.enterErr },
				Hold: func() string {
					if tc.holdAfterPaste && pastes > 0 {
						return "synthetic hold"
					}
					return ""
				},
			}
			got := PasteAndSubmitResult(io, Opts{PasteRetries: 2}, "fixture")
			if pastes != 1 || enters != tc.wantCalls || got.Attempts != tc.wantCalls ||
				got.State != tc.wantState || got.FailureCode != tc.wantCode || got.Submission != tc.wantBoundary {
				t.Fatalf("result=%+v paste=%d Enter=%d", got, pastes, enters)
			}
			if tc.holdAfterPaste && !strings.HasPrefix(got.Evidence, EvidenceHeldBeforeEnter) {
				t.Fatalf("lost post-paste hold: %+v", got)
			}
			pastes, enters = 0, 0
			ok, refused := PasteAndSubmit(io, Opts{}, "fixture")
			if tc.wantState == StateSubmitted {
				if !ok || refused != "" {
					t.Fatal("legacy wrapper lost success", ok, refused)
				}
			} else if ok || refused != tc.wantState {
				t.Fatal("legacy wrapper changed the refusal", ok, refused)
			}
		})
	}
}

func TestSPL008VerifiedHoldKeepsItsEvidenceAndSubmitCount(t *testing.T) {
	io := IO{Capture: func() string { return "" }, Sleep: func() {}, Now: func() int64 { return 1 }}
	io.Paste = func(string) error { return nil }
	io.Enter = func() error { t.Fatal("hold allowed Enter"); return nil }
	for _, afterPaste := range []bool{false, true} {
		holds := 0
		io.Hold = func() string {
			holds++
			if afterPaste && holds == 1 {
				return ""
			}
			return "stale-premise"
		}
		got := Deliver(io, Opts{}, "fixture")
		if got.State != StateRefusedWaiting || got.Attempts != 0 || got.Submission != SubmissionNotSubmitted ||
			strings.HasPrefix(got.Evidence, EvidenceHeldBeforeEnter) != afterPaste {
			t.Fatalf("hold=%t result=%+v", afterPaste, got)
		}
	}
}

func TestSPL008RefusedRetryDoesNotResolveEarlierEnter(t *testing.T) {
	f := &fakeIO{caps: []string{boxDraft(taskText)}}
	io := f.io()
	enter := io.Enter
	io.Enter = func() error {
		if f.enterCalls != 0 {
			return &InputError{Code: InputTargetChanged, Submission: SubmissionNotSubmitted, Cause: errors.New("target changed")}
		}
		return enter()
	}
	got := Deliver(io, Opts{Pane: "%1", HasComposer: true, DeliverTimeout: 6}, taskText)
	if f.enterCalls != 1 || got.Attempts != 1 || got.FailureCode != InputOutcomeUnconfirmed || got.Submission != SubmissionUnknown {
		t.Fatalf("earlier Enter was incorrectly resolved: %+v calls=%d", got, f.enterCalls)
	}
}
