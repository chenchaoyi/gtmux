package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/server"
)

func TestSPL003PhoneClassifiesCapacityAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		code     dispatch.InputFailureCode
		boundary dispatch.Submission
		want     string
	}{
		{dispatch.InputSpoolFull, dispatch.SubmissionNotSubmitted, server.SendInputSpoolFull},
		{dispatch.InputOutcomeUnconfirmed, dispatch.SubmissionUnknown, server.SendOutcomeUnconfirmed},
		{dispatch.InputSessionNotLoaded, dispatch.SubmissionUnknown, server.SendOutcomeUnconfirmed},
	} {
		got := phoneSendFailure(dispatch.Result{State: dispatch.StateFailed, FailureCode: tc.code, Submission: tc.boundary})
		if failure, ok := got.(*server.SendError); !ok || failure.Code != tc.want || failure.Submission != string(tc.boundary) {
			t.Fatalf("%s: got %#v", tc.code, got)
		}
	}
}
