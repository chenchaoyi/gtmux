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
		name, evidence, code string
		refused, nativeState dispatch.State
		question             bool
	}{
		{"verified question", "input-awaiting-confirmation", server.SendInputQuestion, dispatch.StateRefusedWaiting, dispatch.StateRefusedWaiting, true},
		{"claimed but refused question", "codex-input-refused:input-awaiting-confirmation", server.SendInputQuestion, dispatch.StateFailed, dispatch.StateFailed, true},
		{"unknown input", "input-state-unavailable", server.SendInputUnavailable, dispatch.StateRefusedWaiting, dispatch.StateRefusedWaiting, false},
		{"future native policy", "conflicting-native-input-policy", server.SendInputUnavailable, dispatch.StateRefusedWaiting, dispatch.StateRefusedWaiting, false},
		{"question substring is not proof", "future:input-awaiting-confirmation", server.SendInputUnavailable, dispatch.StateRefusedWaiting, dispatch.StateRefusedWaiting, false},
		{"inconsistent result is not proof", "input-awaiting-confirmation", server.SendInputUnavailable, dispatch.StateRefusedWaiting, dispatch.StateFailed, false},
		{"association", "codex-frontend-unavailable", server.SendConversationUnavailable, dispatch.StateRefusedWaiting, dispatch.StateRefusedWaiting, false},
		{"target changed", "expected-conversation-changed", server.SendTargetChanged, dispatch.StateRefusedWaiting, dispatch.StateRefusedWaiting, false},
		{"input target changed", "codex-input-target-changed", server.SendTargetChanged, dispatch.StateRefusedWaiting, dispatch.StateRefusedWaiting, false},
		{"not loaded", "codex-input-refused:input-thread-not-loaded", server.SendSessionNotLoaded, dispatch.StateFailed, dispatch.StateFailed, false},
		{"uncertain", "codex-input-uncertain:timeout", server.SendOutcomeUnconfirmed, dispatch.StateFailed, dispatch.StateFailed, false},
		{"receipt lost", "codex-input-receipt-unconfirmed", server.SendOutcomeUnconfirmed, dispatch.StateFailed, dispatch.StateFailed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := phoneSendRefusal(tc.refused, &dispatch.Result{State: tc.nativeState, Evidence: tc.evidence})
			var got *server.SendError
			if !errors.As(err, &got) || got.Code != tc.code {
				t.Fatal("wrong refusal", err, got)
			}
			if strings.Contains(err.Error(), "is asking something") != tc.question {
				t.Fatal("invented or lost a verified question", err)
			}
			if tc.code == server.SendOutcomeUnconfirmed && !strings.Contains(err.Error(), "will not be replayed") {
				t.Fatal("unknown-outcome advice lost", err)
			}
		})
	}
	for state, code := range map[dispatch.State]string{
		dispatch.StateRefusedWaiting: server.SendInputUnavailable,
		dispatch.StateRefusedDraft:   server.SendDraftPresent,
		dispatch.StateFailed:         server.SendInputUnconfirmed,
	} {
		err := phoneSendRefusal(state, nil)
		var got *server.SendError
		if !errors.As(err, &got) || got.Code != code || strings.Contains(err.Error(), "asking something") {
			t.Fatal("generic state was called a question", state, err)
		}
	}
}
