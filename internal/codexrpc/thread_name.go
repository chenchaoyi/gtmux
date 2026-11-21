package codexrpc

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// SeedThreadName names an exact local thread if its current metadata is unnamed.
// The caller supplies a deadline and checks its current authority at the write.
// Codex returns an existing automatic name and a user name in the same field,
// so any existing name is preserved.
func SeedThreadName(ctx context.Context, target Target, name string, current func() bool) (string, string) {
	if target.HomeID == "" || target.HomeID != transcript.CodexHomeID() ||
		target.BackendPID <= 1 || target.BackendStart == "" ||
		target.ThreadID == "" || len(target.ThreadID) > 256 || strings.ContainsAny(target.ThreadID, "\r\n\x00") ||
		name == "" || current == nil {
		return "", "invalid-title-target"
	}
	c, err := connect(ctx)
	if err != nil {
		return "", "local-proxy-unavailable"
	}
	defer c.close()
	if c.peerPID != target.BackendPID || c.peerStart != target.BackendStart {
		return "", "backend-changed"
	}
	w := newWire(c)
	if reason := w.initialize(target.HomeID); reason != "" {
		return "", reason
	}
	if w.send(2, "thread/read", map[string]any{"threadId": target.ThreadID, "includeTurns": false}) != nil {
		return "", "thread-read-unavailable"
	}
	r, err := w.read(2)
	var data struct {
		Thread struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Ephemeral bool   `json:"ephemeral"`
		} `json:"thread"`
	}
	if err != nil || r.Error != nil || json.Unmarshal(r.Result, &data) != nil || data.Thread.ID != target.ThreadID || data.Thread.Ephemeral {
		return "", "thread-read-unavailable"
	}
	if ctx.Err() != nil || !current() || ctx.Err() != nil {
		return "", "title-target-changed"
	}
	if data.Thread.Name != "" {
		return data.Thread.Name, ""
	}
	if w.send(3, "thread/name/set", map[string]string{"threadId": target.ThreadID, "name": name}) != nil {
		return "", "title-unconfirmed"
	}
	r, err = w.read(3)
	var result map[string]json.RawMessage
	if err != nil || r.Error != nil || json.Unmarshal(r.Result, &result) != nil || result == nil {
		return "", "title-unconfirmed"
	}
	return name, ""
}
