// Package codexbridge observes root-thread identity on one verified local TUI
// connection while forwarding the protocol unchanged. It stores no message bodies.
package codexbridge

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

const observationLimit = 512 << 10

type operation struct {
	initialize bool
	sequence   uint64
}

// Only requests that can establish this client's primary conversation enter
// pending. Catalog, history, turn and global notifications never claim a pane.
type observer struct {
	mu      sync.Mutex
	pending map[string]operation
	home    string
	ready   bool
	latest  uint64
	bind    func(string, string)
}

func (o *observer) request(data []byte) {
	if len(data) > observationLimit {
		return
	}
	var v struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			Ephemeral    bool            `json:"ephemeral"`
			ThreadSource json.RawMessage `json:"threadSource"`
		} `json:"params"`
	}
	if json.Unmarshal(data, &v) != nil || len(v.ID) == 0 || len(v.ID) > 256 {
		return
	}
	op := operation{initialize: v.Method == "initialize"}
	switch v.Method {
	case "initialize":
	case "thread/start", "thread/resume", "thread/fork":
		if v.Params.Ephemeral || len(v.Params.ThreadSource) > 0 && string(v.Params.ThreadSource) != `"user"` && string(v.Params.ThreadSource) != "null" {
			return
		}
	default:
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.pending == nil {
		o.pending = make(map[string]operation)
	}
	if !op.initialize {
		o.latest++
		op.sequence = o.latest
	}
	if len(o.pending) < 64 {
		o.pending[string(v.ID)] = op
	}
}

func (o *observer) response(data []byte) {
	if len(data) > 64<<20 {
		return
	}
	// Most envelopes start with id. Large untracked catalogs take this bounded
	// header path and never become a generic object or retained pending entry.
	var id json.RawMessage
	d := json.NewDecoder(bytes.NewReader(data))
	opening, err := d.Token()
	key, keyErr := d.Token()
	if err == nil && keyErr == nil && opening == json.Delim('{') && key == "id" {
		if d.Decode(&id) != nil {
			return
		}
	} else {
		var envelope struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(data, &envelope) != nil {
			return
		}
		id = envelope.ID
	}
	o.mu.Lock()
	_, pending := o.pending[string(id)]
	o.mu.Unlock()
	if !pending {
		return
	}
	var v struct {
		ID     json.RawMessage `json:"id"`
		Error  json.RawMessage `json:"error"`
		Result struct {
			Home   string `json:"codexHome"`
			Thread struct {
				ID        string          `json:"id"`
				Cwd       string          `json:"cwd"`
				Ephemeral bool            `json:"ephemeral"`
				Source    json.RawMessage `json:"source"`
			} `json:"thread"`
		} `json:"result"`
	}
	if json.Unmarshal(data, &v) != nil {
		return
	}
	o.mu.Lock()
	op, ok := o.pending[string(v.ID)]
	delete(o.pending, string(v.ID))
	if !ok || len(v.Error) > 0 && string(v.Error) != "null" {
		o.mu.Unlock()
		return
	}
	if op.initialize {
		o.ready = v.Result.Home != "" && transcript.CodexHomeIDForPath(v.Result.Home) == o.home
		o.mu.Unlock()
		return
	}
	t := v.Result.Thread
	var source string
	_ = json.Unmarshal(t.Source, &source)
	valid := o.ready && op.sequence == o.latest && t.ID != "" && len(t.ID) <= 256 && !t.Ephemeral && (source == "cli" || source == "vscode")
	if valid {
		o.bind(t.ID, t.Cwd)
	}
	o.mu.Unlock()
}
