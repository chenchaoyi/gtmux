package agentclient

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/state"
)

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
}

type Receipt struct {
	Version   int    `json:"version"`
	ID        string `json:"request_id"`
	Instance  string `json:"instance"`
	HomeID    string `json:"home_id"`
	SessionID string `json:"session_id"`
	PID       int    `json:"pid"`
	State     string `json:"state"`
	Error     string `json:"error"`
	CreatedAt int64  `json:"created_at"`
}

func InputDir() string            { return filepath.Join(state.Dir(), "terminal-input") }
func (r Request) Command() string { return "/gtmux-send " + r.ID }

// Prepare keeps a durable retry key. A claimed request with no outcome remains
// uncertain; neither a timeout nor a server restart authorizes duplicate input.
func Prepare(a Attachment, text, deliveryID string, force bool, now time.Time) (Request, error) {
	if !a.InputReady || len(text) > 1<<20 || strings.TrimSpace(text) == "" {
		return Request{}, errors.New("terminal input is unavailable or exceeds its limit")
	}
	lease, err := state.TryLease("terminal-input")
	if err != nil {
		return Request{}, err
	}
	defer lease.Close()
	dir := InputDir()
	if err := state.MkdirAll(dir, 0700); err != nil {
		return Request{}, err
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0077 != 0 {
		return Request{}, errors.New("untrusted terminal input directory")
	}
	if err := pruneInput(dir, now); err != nil {
		return Request{}, err
	}
	textSum := sha256.Sum256([]byte(text))
	textHash := hex.EncodeToString(textSum[:])
	seed := a.HomeID + "\x00" + a.SessionID + "\x00" + a.Instance + "\x00" + fmt.Sprint(a.BindingEpoch) + "\x00" + text
	if deliveryID != "" {
		seed = "delivery\x00" + a.Pane + "\x00" + deliveryID
	}
	key := sha256.Sum256([]byte(seed))
	index := filepath.Join(dir, hex.EncodeToString(key[:16])+".index.json")
	var old Request
	if !force && readOwnedJSON(index, &old, 8192) == nil && instancePattern.MatchString(old.ID) {
		if old.TextHash != textHash {
			return Request{}, errors.New("delivery ID was already used for another message")
		}
		old.Text = text
		receipt, known := ReadReceipt(old)
		if known {
			switch receipt.State {
			case "accepted":
				if deliveryID != "" || now.UnixMilli()-old.CreatedAt <= 15000 {
					return old, nil
				}
			case "uncertain":
				return Request{}, errors.New("previous input has an uncertain outcome; do not retry automatically")
			case "refused":
				return Request{}, errors.New(receipt.Error)
			}
		} else {
			if _, err := os.Lstat(filepath.Join(dir, old.ID+".claimed.json")); err == nil {
				return Request{}, errors.New("previous input was claimed without a receipt; do not retry automatically")
			}
			if now.UnixMilli()-old.CreatedAt >= 0 && now.UnixMilli()-old.CreatedAt <= 15000 {
				return old, nil
			}
		}
		if deliveryID != "" && (old.SessionID != a.SessionID || old.Instance != a.Instance || old.BindingEpoch != a.BindingEpoch || old.HomeID != a.HomeID) {
			return Request{}, errors.New("delivery target changed; the same delivery ID cannot retarget input")
		}
	}
	// Bound the new spool without following directory links or allocating its
	// complete contents. Existing unresolved requests are never silently deleted.
	f, err := os.Open(dir)
	if err != nil {
		return Request{}, err
	}
	entries, readErr := f.ReadDir(4097)
	_ = f.Close()
	if readErr != nil && readErr != io.EOF {
		return Request{}, readErr
	}
	if len(entries) > 4096 {
		return Request{}, errors.New("terminal input spool is full")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Request{}, err
	}
	r := Request{Version: ProtocolVersion, ID: hex.EncodeToString(nonce[:]), Instance: a.Instance, HomeID: a.HomeID, SessionID: a.SessionID, PID: a.PID, Pane: a.Pane, ServerEpoch: a.ServerEpoch, BindingEpoch: a.BindingEpoch, CreatedAt: now.UnixMilli(), Text: text, TextHash: textHash, DurableRetry: deliveryID != ""}
	b, _ := json.Marshal(r)
	pending := filepath.Join(dir, r.ID+".pending.json")
	if err := writeExclusive(pending, b); err != nil {
		return Request{}, err
	}
	metadata := r
	metadata.Text = ""
	indexBytes, _ := json.Marshal(metadata)
	if err := atomicPrivate(index, indexBytes); err != nil {
		_ = os.Remove(pending)
		return Request{}, err
	}
	return r, nil
}

func writeExclusive(path string, data []byte) error {
	f, err := state.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
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
	var value Receipt
	if !instancePattern.MatchString(r.ID) || readOwnedJSON(filepath.Join(InputDir(), r.ID+".receipt.json"), &value, 8192) != nil ||
		value.Version != ProtocolVersion || value.ID != r.ID || value.Instance != r.Instance || value.PID != r.PID || value.HomeID != r.HomeID {
		return Receipt{}, false
	}
	if value.State == "accepted" && value.SessionID != r.SessionID {
		return Receipt{}, false
	}
	return value, value.State == "accepted" || value.State == "refused" || value.State == "uncertain"
}

// WrapIO keeps the ordinary draft/fragment/interlock pipeline. Only a known
// request token is rendered as the submitted body for draft comparisons; an
// Enter succeeds solely on the receiving native command's addressed receipt.
func WrapIO(pane string, io dispatch.IO, lookup func(string) (Attachment, bool)) dispatch.IO {
	var request *Request
	var beginErr error
	begin, capture, color, paste, enter, eventRead := io.Begin, io.Capture, io.CaptureColor, io.Paste, io.Enter, io.Events
	io.Begin = func(opts dispatch.Opts, text string) {
		if begin != nil {
			begin(opts, text)
		}
		a, ok := lookup(pane)
		if !ok || !a.InputReady {
			beginErr = errors.New("DeepSeek Harness terminal bridge is not loaded or its input grant is unavailable")
			return
		}
		r, err := Prepare(a, text, opts.DeliveryID, opts.Force && opts.DeliveryID == "", time.Now())
		beginErr = err
		if err == nil {
			request = &r
		}
	}
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
		a, ok := lookup(pane)
		if !ok || !a.InputReady {
			return errors.New("DeepSeek Harness terminal bridge is not loaded or its input grant is unavailable")
		}
		if beginErr != nil {
			return beginErr
		}
		if request == nil {
			return errors.New("native terminal request was not prepared")
		}
		if request.SessionID != a.SessionID || request.Instance != a.Instance || request.BindingEpoch != a.BindingEpoch {
			return errors.New("terminal conversation changed before paste")
		}
		if receipt, ok := ReadReceipt(*request); ok && receipt.State == "accepted" {
			return nil
		}
		if !prompt.IsComposerReady(rawFrame(), "dsh") {
			return errors.New("DeepSeek Harness terminal composer is unavailable; close menus or expanded editors and use insert mode")
		}
		return paste(request.Command())
	}
	io.Enter = func() error {
		if request == nil {
			return errors.New("native terminal request was not prepared")
		}
		if receipt, ok := ReadReceipt(*request); ok {
			if receipt.State == "accepted" {
				return nil
			}
			return errors.New(receipt.Error)
		}
		a, ok := lookup(pane)
		if !ok || !a.InputReady || a.SessionID != request.SessionID || a.Instance != request.Instance || a.BindingEpoch != request.BindingEpoch {
			return errors.New("terminal conversation changed before submission")
		}
		if !prompt.IsComposerReady(rawFrame(), "dsh") {
			return errors.New("DeepSeek Harness terminal composer changed before submission")
		}
		if err := enter(); err != nil {
			return err
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if receipt, ok := ReadReceipt(*request); ok {
				if receipt.State == "accepted" {
					return nil
				}
				return errors.New(receipt.Error)
			}
			time.Sleep(25 * time.Millisecond)
		}
		return errors.New("native input acceptance is unconfirmed; retry preserves the same request")
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
