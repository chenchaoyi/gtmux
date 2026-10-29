// Package codexrpc addresses explicit user input to a verified local Codex thread.
// It never discovers conversations, resumes threads or enables remote control.
package codexrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

type Target struct {
	HomeID, ThreadID string
	BackendPID       int
	BackendStart     string
}

type Input struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Path string `json:"path,omitempty"`
}

// Outcome describes transport acceptance, never model execution or completion.
// Once Attempted is true, missing confirmation must not authorize a fresh send.
type Outcome struct {
	Accepted, Attempted, Unsupported bool
	QueueID, Reason                  string
}

type wireResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

type connection struct {
	peerPID   int
	peerStart string
	reader    io.Reader
	writer    io.Writer
	close     func()
}

var connect = connectLocal

// Queue requires the caller's existing identity/access checks and a stable user
// message ID. The configured home is rechecked against the server before input.
func Queue(ctx context.Context, target Target, messageID, text string) Outcome {
	return QueueChecked(ctx, target, messageID, text, nil)
}

// QueueChecked revalidates the caller's exact destination immediately before
// the first mutating write. A false check must have no input side effects.
func QueueChecked(ctx context.Context, target Target, messageID, text string, check func() bool) Outcome {
	return QueueInputs(ctx, target, messageID, []Input{{Type: "text", Text: text}}, check)
}

func QueueInputs(ctx context.Context, target Target, messageID string, inputs []Input, check func() bool) Outcome {
	bytes := 0
	valid := len(inputs) > 0 && len(inputs) <= 17
	for _, input := range inputs {
		switch input.Type {
		case "text":
			valid = valid && input.Text != ""
			bytes += len(input.Text)
		case "localImage":
			valid = valid && filepath.IsAbs(input.Path) && len(input.Path) < 4096
		default:
			valid = false
		}
	}
	if target.HomeID == "" || target.HomeID != transcript.CodexHomeID() || target.ThreadID == "" ||
		len(target.ThreadID) > 256 || messageID == "" || len(messageID) > 256 || !valid || bytes > 1<<20 {
		return Outcome{Reason: "invalid-input-target"}
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c, err := connect(ctx)
	if err != nil {
		return Outcome{Reason: "local-proxy-unavailable"}
	}
	defer c.close()
	if target.BackendPID > 0 && (c.peerPID != target.BackendPID || c.peerStart != target.BackendStart) {
		return Outcome{Reason: "backend-changed"}
	}
	enc := json.NewEncoder(c.writer)
	// These two addressed responses are small; no catalog or history is read.
	dec := json.NewDecoder(io.LimitReader(c.reader, 2<<20))
	send := func(id int, method string, params any) error {
		return enc.Encode(map[string]any{"id": id, "method": method, "params": params})
	}
	read := func(id int) (wireResponse, error) {
		for n := 0; n < 64; n++ {
			var r wireResponse
			if err := dec.Decode(&r); err != nil {
				return r, err
			}
			if r.ID == id {
				return r, nil
			}
		}
		return wireResponse{}, errors.New("response bound exceeded")
	}
	if send(1, "initialize", map[string]any{
		"clientInfo":   map[string]string{"name": "gtmux", "title": "gtmux", "version": "1"},
		"capabilities": map[string]bool{"experimentalApi": true},
	}) != nil {
		return Outcome{Reason: "initialize-unavailable"}
	}
	r, err := read(1)
	if err != nil || r.Error != nil {
		return Outcome{Reason: "initialize-unavailable"}
	}
	var server struct {
		Home string `json:"codexHome"`
	}
	if json.Unmarshal(r.Result, &server) != nil || server.Home == "" || transcript.CodexHomeIDForPath(server.Home) != target.HomeID {
		return Outcome{Reason: "server-home-mismatch"}
	}
	if enc.Encode(map[string]any{"method": "initialized", "params": map[string]any{}}) != nil {
		return Outcome{Reason: "initialize-unavailable"}
	}
	if check != nil && !check() {
		return Outcome{Reason: "target-changed"}
	}
	result := Outcome{Attempted: true, Reason: "acceptance-unconfirmed"}
	if send(2, "thread/queue/add", map[string]any{
		"threadId": target.ThreadID, "clientUserMessageId": messageID,
		"input": inputs,
	}) != nil {
		return result
	}
	r, err = read(2)
	if err != nil {
		return result
	}
	if r.Error != nil {
		// Method-not-found and invalid parameters are pre-execution rejections.
		// A generic server error can follow a persisted enqueue; do not replay it.
		if r.Error.Code == -32601 || r.Error.Code == -32602 {
			return Outcome{Unsupported: r.Error.Code == -32601, Reason: "queue-rejected"}
		}
		return result
	}
	var accepted struct {
		Submission struct {
			ID        string `json:"id"`
			MessageID string `json:"clientUserMessageId"`
		} `json:"queuedSubmission"`
	}
	if json.Unmarshal(r.Result, &accepted) != nil || accepted.Submission.ID == "" || accepted.Submission.MessageID != messageID {
		return result
	}
	return Outcome{Accepted: true, Attempted: true, QueueID: accepted.Submission.ID}
}
