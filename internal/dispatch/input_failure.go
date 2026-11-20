package dispatch

import (
	"errors"
	"strings"
)

// Submission describes what the sender can establish about input mutation.
type Submission string

const (
	SubmissionNotSubmitted Submission = "not-submitted"
	SubmissionUnknown      Submission = "unknown"
)

type InputFailureCode string

const (
	InputSpoolFull          InputFailureCode = "input-spool-full"
	InputQuestion           InputFailureCode = "input-awaiting-confirmation"
	InputUnavailable        InputFailureCode = "input-state-unavailable"
	InputDraftPresent       InputFailureCode = "draft-present"
	InputAssociationMissing InputFailureCode = "conversation-unavailable"
	InputTargetChanged      InputFailureCode = "target-changed"
	InputSessionNotLoaded   InputFailureCode = "session-not-loaded"
	InputOutcomeUnconfirmed InputFailureCode = "outcome-unconfirmed"
	InputUnconfirmed        InputFailureCode = "input-unconfirmed"
)

// InputError lets a transport report a proven refusal even when it has been
// called through Enter. An unresolved earlier request can be unknown with zero
// submit attempts in the current invocation.
type InputError struct {
	Code       InputFailureCode
	Submission Submission
	Cause      error
}

func (e *InputError) Error() string { return e.Cause.Error() }
func (e *InputError) Unwrap() error { return e.Cause }

func InputFailureResult(err error, attempts int) Result {
	result := Result{State: StateFailed, Attempts: attempts, Evidence: err.Error(),
		FailureCode: InputUnavailable, Submission: SubmissionNotSubmitted}
	if attempts > 0 {
		result.FailureCode, result.Submission = InputOutcomeUnconfirmed, SubmissionUnknown
	} else if errors.Is(err, errUnconfirmedPaste) {
		result.FailureCode = InputUnconfirmed
	}
	var failure *InputError
	if errors.As(err, &failure) {
		result.FailureCode, result.Submission = failure.Code, failure.Submission
	}
	return result
}

func draftRefusal(evidence string) Result {
	if strings.HasPrefix(evidence, EvidenceMenuOpen) {
		return waitingRefusal(evidence, InputQuestion)
	}
	return Result{State: StateRefusedDraft, Evidence: evidence, FailureCode: InputDraftPresent, Submission: SubmissionNotSubmitted}
}

func waitingRefusal(evidence string, code InputFailureCode) Result {
	return Result{State: StateRefusedWaiting, Evidence: evidence, FailureCode: code, Submission: SubmissionNotSubmitted}
}

// WithUnverifiedPriorDelivery preserves the current failure reason without
// letting a current refusal settle an earlier request under the same ID.
func (r Result) WithUnverifiedPriorDelivery() Result {
	if !r.Submitted() {
		r.State, r.Submission = StateFailed, SubmissionUnknown
	}
	return r
}

// Submitted also accepts queued receipts and unverified completed Enter calls.
func (r Result) Submitted() bool {
	return r.Delivered || r.State == StateQueued || r.State == StateSubmitted
}
