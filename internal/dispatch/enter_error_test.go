package dispatch

import (
	"errors"
	"testing"
)

func TestDeliveryDoesNotIgnoreNativeSubmissionRefusal(t *testing.T) {
	for _, fast := range []bool{false, true} {
		frame := boxEmpty("ready")
		io := IO{Capture: func() string { return frame },
			Paste: func(text string) error { frame = boxDraft(text); return nil },
			Enter: func() error { frame = boxEmpty(taskText); return errors.New("native target changed") },
			Now:   func() int64 { return 100 }, Sleep: func() {}}
		opts := Opts{Pane: "%1", HasComposer: true}
		if fast {
			if ok, state := PasteAndSubmit(io, opts, taskText); ok || state != StateFailed {
				t.Fatalf("unverified send ignored refusal: %v %s", ok, state)
			}
		} else if result := Deliver(io, opts, taskText); result.Delivered || result.State != StateFailed {
			t.Fatalf("verified send ignored refusal: %+v", result)
		}
	}
}
