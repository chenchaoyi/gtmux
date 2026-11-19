package server

import "errors"

const (
	SendInputSpoolFull          = "input-spool-full"
	SendInputQuestion           = "input-awaiting-confirmation"
	SendInputUnavailable        = "input-state-unavailable"
	SendDraftPresent            = "draft-present"
	SendConversationUnavailable = "conversation-unavailable"
	SendTargetChanged           = "target-changed"
	SendSessionNotLoaded        = "session-not-loaded"
	SendOutcomeUnconfirmed      = "outcome-unconfirmed"
	SendInputUnconfirmed        = "input-unconfirmed"
)

// SendError adds optional recovery context without changing legacy error text.
type SendError struct {
	Code, Message, Submission string
}

func (e *SendError) Error() string           { return e.Message }
func (e *SendError) SendFailureCode() string { return e.Code }

func sendFailureBody(err error, sendID string) map[string]string {
	body := errBody("send failed: " + err.Error())
	var coded interface{ SendFailureCode() string }
	if errors.As(err, &coded) {
		switch code := coded.SendFailureCode(); code {
		case SendInputSpoolFull, SendInputQuestion, SendInputUnavailable, SendDraftPresent,
			SendConversationUnavailable, SendTargetChanged, SendSessionNotLoaded,
			SendOutcomeUnconfirmed, SendInputUnconfirmed:
			body["code"] = code
		}
	}
	var result *SendError
	if errors.As(err, &result) {
		switch result.Submission {
		case "not-submitted", "unknown":
			body["submission"] = result.Submission
			if sendID != "" {
				body["send_id"] = sendID
			}
		}
	}
	return body
}
