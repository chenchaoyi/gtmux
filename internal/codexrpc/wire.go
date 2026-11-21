package codexrpc

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

type wire struct {
	enc *json.Encoder
	dec *json.Decoder
}

func newWire(c connection) wire {
	return wire{json.NewEncoder(c.writer), json.NewDecoder(io.LimitReader(c.reader, 2<<20))}
}

func (w wire) send(id int, method string, params any) error {
	return w.enc.Encode(map[string]any{"id": id, "method": method, "params": params})
}

func (w wire) read(id int) (wireResponse, error) {
	for range 64 {
		var r wireResponse
		if err := w.dec.Decode(&r); err != nil {
			return r, err
		}
		if r.ID == id {
			return r, nil
		}
	}
	return wireResponse{}, errors.New("response bound exceeded")
}

func (w wire) initialize(homeID string) string {
	if w.send(1, "initialize", map[string]any{
		"clientInfo":   map[string]string{"name": "gtmux", "title": "gtmux", "version": "1"},
		"capabilities": map[string]bool{"experimentalApi": true},
	}) != nil {
		return "initialize-unavailable"
	}
	r, err := w.read(1)
	if err != nil || r.Error != nil {
		return "initialize-unavailable"
	}
	var server struct {
		Home string `json:"codexHome"`
	}
	if json.Unmarshal(r.Result, &server) != nil || server.Home == "" || transcript.CodexHomeIDForPath(server.Home) != homeID {
		return "server-home-mismatch"
	}
	if w.enc.Encode(map[string]any{"method": "initialized", "params": map[string]any{}}) != nil {
		return "initialize-unavailable"
	}
	return ""
}
