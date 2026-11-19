package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/server"
)

func TestSFR001PhoneRefusalNeedsExplicitNativeQuestion(t *testing.T) {
	for _, tc := range []struct {
		name, evidence string
		code           dispatch.InputFailureCode
		boundary       dispatch.Submission
		want           string
		question       bool
		native         bool
	}{
		{"verified question", "input-awaiting-confirmation", dispatch.InputQuestion, dispatch.SubmissionNotSubmitted, server.SendInputQuestion, true, true},
		{"unknown wins", "input-awaiting-confirmation", dispatch.InputQuestion, dispatch.SubmissionUnknown, server.SendOutcomeUnconfirmed, false, true},
		{"evidence substring is not proof", "future:input-awaiting-confirmation", dispatch.InputUnavailable, dispatch.SubmissionNotSubmitted, server.SendInputUnavailable, false, false},
		{"association", "codex-frontend-unavailable", dispatch.InputAssociationMissing, dispatch.SubmissionNotSubmitted, server.SendConversationUnavailable, false, false},
		{"screen menu", dispatch.EvidenceMenuOpen, dispatch.InputQuestion, dispatch.SubmissionNotSubmitted, server.SendInputUnavailable, false, false},
		{"not loaded", "private transport details", dispatch.InputSessionNotLoaded, dispatch.SubmissionNotSubmitted, server.SendSessionNotLoaded, false, false},
		{"draft", "private transport details", dispatch.InputDraftPresent, dispatch.SubmissionNotSubmitted, server.SendDraftPresent, false, false},
		{"fragment", "private transport details", dispatch.InputUnconfirmed, dispatch.SubmissionNotSubmitted, server.SendInputUnconfirmed, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := phoneSendFailure(dispatch.Result{State: dispatch.StateFailed, Evidence: tc.evidence, FailureCode: tc.code, Submission: tc.boundary, NativeQuestion: tc.native})
			var got *server.SendError
			if !errors.As(err, &got) || got.Code != tc.want || got.Submission != string(tc.boundary) {
				t.Fatal("wrong refusal", err, got)
			}
			if strings.Contains(err.Error(), "is asking something") != tc.question || strings.Contains(err.Error(), "private transport details") {
				t.Fatal("unsafe or inaccurate diagnostic", err)
			}
		})
	}
}
