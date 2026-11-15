package app

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/server"
)

// phoneSendRefusal explains one refused attempt; it does not change input policy.
func phoneSendRefusal(refused dispatch.State, native *dispatch.Result) error {
	fail := func(code, text string) error { return &server.SendError{Code: code, Message: text} }
	if native != nil {
		evidence := native.Evidence
		if en, _ := associationRefusalMessage(evidence); en != "" {
			code := server.SendConversationUnavailable
			if evidence != "codex-frontend-unavailable" {
				code = server.SendTargetChanged
			}
			return fail(code, en)
		}
		if strings.Contains(evidence, "input-thread-not-loaded") {
			return fail(server.SendSessionNotLoaded, "not sent: the session is not loaded; open it on the Mac first")
		}
		if strings.Contains(evidence, "uncertain") || strings.Contains(evidence, "receipt-unconfirmed") {
			return fail(server.SendOutcomeUnconfirmed, "send outcome unconfirmed; inspect the conversation before retrying; the same message ID will not be replayed")
		}
		question := refused == native.State && ((refused == dispatch.StateRefusedWaiting && evidence == "input-awaiting-confirmation") ||
			(refused == dispatch.StateFailed && evidence == "codex-input-refused:input-awaiting-confirmation"))
		if question {
			return fail(server.SendInputQuestion, "not sent: the agent is asking something (a permission prompt or a question); answer it first")
		}
	}
	if refused == dispatch.StateRefusedDraft {
		return fail(server.SendDraftPresent, "not sent: that pane has unsent text in its input box; clear it, or send from the Mac")
	}
	if refused == dispatch.StateRefusedWaiting {
		return fail(server.SendInputUnavailable, "not sent: the session's input state could not be verified; inspect the session and try again")
	}
	return fail(server.SendInputUnconfirmed, "not confirmed: the pane's input box did not settle on the full message")
}
