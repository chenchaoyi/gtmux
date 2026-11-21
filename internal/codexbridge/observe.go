// Package codexbridge observes root-thread identity on one verified local TUI
// connection. It can reflect a confirmed HQ name in a root response and adapt
// the worktree-trust read documented in trust_config.go. It stores no message bodies.
package codexbridge

import (
	"bytes"
	"encoding/json"
	"sync"
	"sync/atomic"

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
	mu                sync.Mutex
	pending           map[string]operation
	home              string
	ready             bool
	homeReady         atomic.Bool
	latest            uint64
	initializeVersion uint64
	bind              func(string, string)
	seedRootName      func(string) string
	selectRoot        func()
	resetHome         func()
}

func (o *observer) request(data []byte) {
	if len(data) > observationLimit {
		return
	}
	var v struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			Ephemeral bool `json:"ephemeral"`
		} `json:"params"`
	}
	if json.Unmarshal(data, &v) != nil || len(v.ID) == 0 || len(v.ID) > 256 {
		return
	}
	op := operation{initialize: v.Method == "initialize"}
	switch v.Method {
	case "initialize":
	case "thread/start", "thread/resume", "thread/fork":
		if v.Params.Ephemeral {
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
	o.latest++
	op.sequence = o.latest
	if op.initialize {
		o.initializeVersion = op.sequence
		o.ready = false
		o.homeReady.Store(false)
		if o.resetHome != nil {
			o.resetHome()
		}
	} else {
		if o.selectRoot != nil {
			o.selectRoot()
		}
	}
	if len(o.pending) < 64 {
		o.pending[string(v.ID)] = op
	}
}

func (o *observer) response(data []byte) { o.rootResponse(data) }

func (o *observer) rootResponse(data []byte) []byte {
	if len(data) > 64<<20 {
		return data
	}
	// Most envelopes start with id. Large untracked catalogs take this bounded
	// header path and never become a generic object or retained pending entry.
	var id json.RawMessage
	d := json.NewDecoder(bytes.NewReader(data))
	opening, err := d.Token()
	key, keyErr := d.Token()
	if err == nil && keyErr == nil && opening == json.Delim('{') && key == "id" {
		if d.Decode(&id) != nil {
			return data
		}
	} else {
		var envelope struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(data, &envelope) != nil {
			return data
		}
		id = envelope.ID
	}
	o.mu.Lock()
	_, pending := o.pending[string(id)]
	o.mu.Unlock()
	if !pending {
		return data
	}
	var v struct {
		ID     json.RawMessage `json:"id"`
		Error  json.RawMessage `json:"error"`
		Result struct {
			Home   string `json:"codexHome"`
			Thread struct {
				ID             string          `json:"id"`
				ParentThreadID string          `json:"parentThreadId"`
				Cwd            string          `json:"cwd"`
				Name           string          `json:"name"`
				Ephemeral      bool            `json:"ephemeral"`
				Source         json.RawMessage `json:"source"`
			} `json:"thread"`
		} `json:"result"`
	}
	if json.Unmarshal(data, &v) != nil {
		return data
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	op, ok := o.pending[string(v.ID)]
	delete(o.pending, string(v.ID))
	if !ok || len(v.Error) > 0 && string(v.Error) != "null" {
		return data
	}
	if op.initialize {
		if op.sequence == o.initializeVersion {
			o.ready = v.Result.Home != "" && transcript.CodexHomeIDForPath(v.Result.Home) == o.home
			o.homeReady.Store(o.ready)
		}
		return data
	}
	t := v.Result.Thread
	var source string
	_ = json.Unmarshal(t.Source, &source)
	valid := o.ready && op.sequence == o.latest && t.ID != "" && len(t.ID) <= 256 && !t.Ephemeral && t.ParentThreadID == "" && (source == "cli" || source == "vscode")
	if valid {
		if t.Name == "" && o.seedRootName != nil {
			if name := o.seedRootName(t.ID); name != "" {
				data = withRootName(data, name)
			}
		}
		o.bind(t.ID, t.Cwd)
	}
	return data
}

func (o *observer) homeVerified() bool { return o.homeReady.Load() }
