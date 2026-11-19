package agentclient

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/state"
)

var ErrInputTargetChanged = errors.New("delivery target changed; the same delivery ID cannot retarget input")

type Request struct {
	Version      int    `json:"version"`
	ID           string `json:"request_id"`
	Instance     string `json:"instance"`
	HomeID       string `json:"home_id"`
	SessionID    string `json:"session_id"`
	PID          int    `json:"pid"`
	Pane         string `json:"pane"`
	ServerEpoch  string `json:"server_epoch"`
	BindingEpoch int64  `json:"binding_epoch"`
	CreatedAt    int64  `json:"created_at"`
	Text         string `json:"text"`
	TextHash     string `json:"text_hash,omitempty"`
	DurableRetry bool   `json:"durable_retry,omitempty"`
	IndexKey     string `json:"index_key,omitempty"`
	store        inputStore
}

type Receipt struct {
	Version   int    `json:"version"`
	ID        string `json:"request_id"`
	Instance  string `json:"instance"`
	HomeID    string `json:"home_id"`
	SessionID string `json:"session_id"`
	PID       int    `json:"pid"`
	State     string `json:"state"`
	Delivery  string `json:"delivery,omitempty"`
	Error     string `json:"error"`
	CreatedAt int64  `json:"created_at"`
}

func (r Request) Command() string { return "/gtmux-send " + r.ID }

func (r Request) persisted() Request {
	r.store = inputStore{}
	return r
}

// MatchesTarget compares the complete prepared address, not just a reused pane.
// A historical accepted receipt may be read without matching the current target,
// but an unclaimed request must never acquire authority over a different target.
func (r Request) MatchesTarget(a Attachment) bool {
	return r.Version == a.Version && r.Instance == a.Instance && r.HomeID == a.HomeID &&
		r.SessionID == a.SessionID && r.PID == a.PID && r.Pane == a.Pane &&
		r.ServerEpoch == a.ServerEpoch && r.BindingEpoch == a.BindingEpoch
}

func deliveryIndex(pane, deliveryID string) string {
	sum := sha256.Sum256([]byte("delivery\x00" + pane + "\x00" + deliveryID))
	return hex.EncodeToString(sum[:16]) + ".index.json"
}

// TerminalFallbackError prevents a durable native ID from becoming a raw
// terminal send when its native route is no longer selected. A pending index
// still belongs to that route: a receiver could claim it independently.
func TerminalFallbackError(pane, deliveryID string) error {
	if deliveryID == "" || terminalSendUnbound(pane, deliveryID) {
		return nil
	}
	return inputUnknown(ErrInputUncertain)
}

func terminalSendUnbound(pane, deliveryID string) bool {
	root := state.Dir()
	store, err := resolveInputStore(root)
	if err != nil {
		return false
	}
	if store.directory == (inputIdentity{}) {
		return true
	}
	queue, err := openInputQueue(store, syncInput)
	if err != nil {
		return false
	}
	defer queue.dir.Close()
	if !queue.missing(deliveryIndex(pane, deliveryID)) {
		return false
	}
	current, err := resolveInputStore(root)
	return err == nil && current == store
}

// PriorDeliveryUnknown is a conservative read-only check before the current
// attempt. Even an accepted old delivery can make a new refusal misleading
// when the caller cannot return that acceptance through the current route.
func PriorDeliveryUnknown(pane, deliveryID string) bool {
	if deliveryID == "" {
		return false
	}
	store, err := resolveInputStore(state.Dir())
	if err != nil {
		return true
	}
	if store.directory == (inputIdentity{}) {
		return false
	}
	queue, err := openInputQueue(store, syncInput)
	if err != nil {
		return true
	}
	defer queue.dir.Close()
	data, err := queue.read(deliveryIndex(pane, deliveryID), 8192)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	var prior Request
	if err != nil || decodeInputRecord(data, &prior, requestFields) != nil ||
		prior.Pane != pane || prior.Version != ProtocolVersion || !instancePattern.MatchString(prior.ID) {
		return true
	}
	receipt, err := queue.committedReceipt(prior)
	if err == nil {
		return receipt.State != "refused"
	}
	if errors.Is(err, ErrInputUncertain) || !errors.Is(err, os.ErrNotExist) {
		return true
	}
	body, err := queue.read(prior.ID+".pending.json", 2<<20)
	return err != nil || !matchesInputBody(body, prior) ||
		!queue.missing(prior.ID+".claimed.json") || !queue.missing(prior.ID+".receipt.json")
}

// Prepare keeps a durable retry key. A claimed request with no outcome remains
// uncertain; neither a timeout nor a server restart authorizes duplicate input.
func Prepare(a Attachment, text, deliveryID string, force bool, now time.Time) (Request, error) {
	intent, err := StageInput(a, text, deliveryID, force, now)
	if err != nil {
		return Request{}, err
	}
	return intent.Publish(now)
}

func atomicPrivate(path string, data []byte) error {
	f, err := state.CreateTemp(filepath.Dir(path), ".gtmux-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func ReadReceipt(r Request) (Receipt, bool) {
	return readCommittedInputReceipt(r, syncInput)
}

func readCommittedInputReceipt(r Request, ops inputDurability) (Receipt, bool) {
	store, err := resolveInputStore(state.Dir())
	if err != nil || r.store.path != "" && r.store != store {
		return Receipt{}, false
	}
	queue, err := openInputQueue(store, ops)
	if err != nil {
		return Receipt{}, false
	}
	defer queue.dir.Close()
	receipt, err := queue.committedReceipt(r)
	if err != nil {
		return Receipt{}, false
	}
	current, err := resolveInputStore(state.Dir())
	return receipt, err == nil && current == store
}

// Capacity accounting reads a snapshot. It does not acknowledge or retire it.
func readInputReceipt(dir, id string) (Receipt, bool) {
	var value Receipt
	if !instancePattern.MatchString(id) || readInputRecord(filepath.Join(dir, id+".receipt.json"), &value, 8192, receiptFields) != nil {
		return Receipt{}, false
	}
	return value, validInputReceipt(value, id)
}

func validInputReceipt(value Receipt, id string) bool {
	if value.Version != ProtocolVersion || value.ID != id || value.CreatedAt < 0 {
		return false
	}
	if value.Delivery != "" && (value.State != "accepted" ||
		(value.Delivery != "active-turn" && value.Delivery != "started-turn" && value.Delivery != "wake-accepted")) {
		return false
	}
	return value.State == "accepted" || value.State == "refused" || value.State == "uncertain"
}

func (r Receipt) matchesRequest(request Request) bool {
	// Older DSH refusals can name the mismatched invocation session. Only an
	// acceptance can establish delivery to the original conversation.
	return r.ID == request.ID && r.Instance == request.Instance && r.PID == request.PID && r.HomeID == request.HomeID &&
		(r.State != "accepted" || r.SessionID == request.SessionID)
}

func inputRefusal(code dispatch.InputFailureCode, cause error) error {
	return &dispatch.InputError{Code: code, Submission: dispatch.SubmissionNotSubmitted, Cause: cause}
}

func inputUnknown(cause error) error {
	return &dispatch.InputError{Code: dispatch.InputOutcomeUnconfirmed, Submission: dispatch.SubmissionUnknown, Cause: cause}
}

func inputPreparationError(err error) error {
	switch {
	case errors.Is(err, ErrInputFull):
		return inputRefusal(dispatch.InputSpoolFull, err)
	case errors.Is(err, ErrInputUncertain):
		return inputUnknown(err)
	case errors.Is(err, ErrInputTargetChanged):
		return inputRefusal(dispatch.InputTargetChanged, err)
	default:
		return inputRefusal(dispatch.InputUnavailable, err)
	}
}

func settledTerminalInput(r Request) (bool, error) {
	if receipt, ok := ReadReceipt(r); ok {
		switch receipt.State {
		case "accepted":
			return true, nil
		case "refused":
			return false, inputRefusal(dispatch.InputUnavailable, errors.New(receipt.Error))
		default:
			return false, inputUnknown(ErrInputUncertain)
		}
	}
	return false, nil
}

// WrapIO keeps the ordinary draft/fragment/interlock pipeline. Only a known
// request token is rendered as the submitted body for draft comparisons; an
// Enter succeeds solely on the receiving native command's addressed receipt.
func WrapIO(pane string, io dispatch.IO, lookup func(string) (Attachment, bool)) dispatch.IO {
	var request *Request
	var intent InputIntent
	var beginErr error
	begin, capture, color, paste, enter, eventRead := io.Begin, io.Capture, io.CaptureColor, io.Paste, io.Enter, io.Events
	io.Begin = func(opts dispatch.Opts, text string) {
		request = nil
		if begin != nil {
			begin(opts, text)
		}
		a, ok := lookup(pane)
		if !ok || !a.InputReady {
			beginErr = inputRefusal(dispatch.InputUnavailable, errors.New("DeepSeek Harness terminal bridge is not loaded or its input grant is unavailable"))
			return
		}
		if !opts.MatchesTarget(a.SessionID, a.HomeID, a.Instance, a.PID) {
			beginErr = inputRefusal(dispatch.InputTargetChanged, errors.New("expected conversation changed before preparation"))
			return
		}
		plan, err := StageInput(a, text, opts.DeliveryID, opts.Force && opts.DeliveryID == "", time.Now())
		beginErr = nil
		if err != nil {
			beginErr = inputPreparationError(err)
			return
		}
		intent = plan
		request = &plan.Request
	}
	io.PriorDeliveryUnknown = func(opts dispatch.Opts) bool {
		return PriorDeliveryUnknown(pane, opts.DeliveryID)
	}
	io.PreflightError = func() error { return beginErr }
	rawFrame := func() string {
		if color != nil {
			return color()
		}
		return capture()
	}
	io.Accepted = func() bool {
		if request == nil {
			return false
		}
		r, known := ReadReceipt(*request)
		return known && r.State == "accepted"
	}
	translate := func(frame string) string {
		if request != nil {
			return strings.ReplaceAll(frame, request.Command(), request.Text)
		}
		return frame
	}
	io.Capture = func() string { return translate(dstFrame(capture(), false)) }
	if color != nil {
		io.CaptureColor = func() string { return translate(dstFrame(color(), true)) }
	}
	io.Paste = func(text string) error {
		if beginErr != nil {
			return beginErr
		}
		a, ok := lookup(pane)
		if !ok || !a.InputReady {
			return inputRefusal(dispatch.InputUnavailable, errors.New("DeepSeek Harness terminal bridge is not loaded or its input grant is unavailable"))
		}
		if request == nil {
			return inputRefusal(dispatch.InputUnavailable, errors.New("native terminal request was not prepared"))
		}
		if !request.MatchesTarget(a) {
			return inputRefusal(dispatch.InputTargetChanged, errors.New("terminal conversation changed before paste"))
		}
		if err := request.CheckInputStore(); err != nil {
			return inputRefusal(dispatch.InputUnavailable, err)
		}
		if a.InputStoreGeneration != request.store.token {
			return inputRefusal(dispatch.InputUnavailable, ErrInputStoreUnavailable)
		}
		if accepted, err := settledTerminalInput(*request); accepted || err != nil {
			return err
		}
		if !missingInputFile(request.store.path, request.ID, ".receipt.json") ||
			!missingInputFile(request.store.path, request.ID, ".claimed.json") {
			return inputUnknown(ErrInputUncertain)
		}
		if !prompt.IsComposerReady(rawFrame(), "dsh") {
			return inputRefusal(dispatch.InputUnavailable, errors.New("DeepSeek Harness terminal composer is unavailable; close menus or expanded editors and use insert mode"))
		}
		return paste(request.Command())
	}
	io.Enter = func() error {
		if request == nil {
			return inputRefusal(dispatch.InputUnavailable, errors.New("native terminal request was not prepared"))
		}
		if accepted, err := settledTerminalInput(*request); accepted || err != nil {
			return err
		}
		a, ok := lookup(pane)
		if !ok || !a.InputReady {
			return inputRefusal(dispatch.InputUnavailable, errors.New("DeepSeek Harness terminal bridge is not loaded or its input grant is unavailable"))
		}
		if !request.MatchesTarget(a) {
			return inputRefusal(dispatch.InputTargetChanged, errors.New("terminal conversation changed before submission"))
		}
		if err := request.CheckInputStore(); err != nil {
			return inputRefusal(dispatch.InputUnavailable, err)
		}
		if a.InputStoreGeneration != request.store.token {
			return inputRefusal(dispatch.InputUnavailable, ErrInputStoreUnavailable)
		}
		if !missingInputFile(request.store.path, request.ID, ".receipt.json") ||
			!missingInputFile(request.store.path, request.ID, ".claimed.json") {
			return inputUnknown(ErrInputUncertain)
		}
		if !prompt.IsComposerReady(rawFrame(), "dsh") {
			return inputRefusal(dispatch.InputUnavailable, errors.New("DeepSeek Harness terminal composer changed before submission"))
		}
		published, err := intent.Publish(time.Now())
		if err != nil {
			return inputPreparationError(err)
		}
		request = &published
		if accepted, err := settledTerminalInput(*request); accepted || err != nil {
			return err
		}
		if err := enter(); err != nil {
			return inputUnknown(err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if accepted, err := settledTerminalInput(*request); accepted || err != nil {
				return err
			}
			time.Sleep(25 * time.Millisecond)
		}
		return inputUnknown(ErrInputUncertain)
	}
	io.Events = func(since int64) []dispatch.Ev {
		if request != nil {
			if receipt, ok := ReadReceipt(*request); ok && receipt.State == "accepted" {
				return []dispatch.Ev{{Kind: dispatch.EvSubmit, Head: dispatch.NormalizeNeedle(request.Text), Ts: time.Now().Unix()}}
			}
			return nil
		}
		if eventRead != nil {
			return eventRead(since)
		}
		return nil
	}
	return io
}
