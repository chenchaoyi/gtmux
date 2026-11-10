package codexbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const trustConfigLimit = 1 << 20
const trustConfigBudget = 250 * time.Millisecond

type trustRead struct {
	cwd string
	at  time.Time
}

// The local TUI lacks its embedded worktree fallback when a verified frontend
// uses an explicit Unix endpoint. This adapter mirrors only an existing backend
// root trust decision; it never writes config or accepts a new trust decision.
type trustConfig struct {
	ctx          context.Context
	homeVerified func() bool
	read         func(context.Context, string) (worktreeTrustProof, bool)
	now          func() time.Time
	mu           sync.Mutex
	pending      map[string]trustRead
	seen         map[string]bool
	proofSlot    chan struct{}
}

func newTrustConfig(ctx context.Context, verified func() bool) *trustConfig {
	return &trustConfig{ctx: ctx, homeVerified: verified, read: readWorktreeTrust, now: time.Now, pending: map[string]trustRead{}, seen: map[string]bool{}, proofSlot: make(chan struct{}, 1)}
}

func trustID(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || len(raw) > 256 {
		return "", false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return "s:" + text, len(text) > 0 && len(text) <= 128
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "", false
	}
	for i, c := range raw {
		if c < '0' || c > '9' {
			if i != 0 || c != '-' {
				return "", false
			}
		}
	}
	if !json.Valid(raw) {
		return "", false
	}
	return "n:" + string(raw), true
}

func (t *trustConfig) request(data []byte) {
	if len(data) > trustConfigLimit || t.ctx.Err() != nil || !t.homeVerified() {
		return
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(data, &envelope) != nil {
		return
	}
	var method string
	if json.Unmarshal(envelope["method"], &method) != nil || method != "config/read" {
		return
	}
	id, ok := trustID(envelope["id"])
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.seen[id] {
		delete(t.pending, id)
		return
	} // never correlate a reused ID
	if len(t.seen) >= 64 {
		return
	}
	t.seen[id] = true
	var params map[string]json.RawMessage
	if json.Unmarshal(envelope["params"], &params) != nil {
		return
	}
	for key := range params {
		if key != "cwd" && key != "includeLayers" {
			return
		}
	}
	var cwd string
	var layers bool
	if json.Unmarshal(params["cwd"], &cwd) != nil || !filepath.IsAbs(cwd) || filepath.Clean(cwd) != cwd || len(cwd) > 4096 || strings.ContainsAny(cwd, "\x00\r\n") || json.Unmarshal(params["includeLayers"], &layers) != nil || !layers {
		return
	}
	now := t.now()
	for key, entry := range t.pending {
		if now.Sub(entry.at) > 5*time.Second {
			delete(t.pending, key)
		}
	}
	if len(t.pending) >= 8 {
		return
	}
	t.pending[id] = trustRead{cwd, now}
}

func (t *trustConfig) response(data []byte) []byte {
	if len(data) > trustConfigLimit || t.ctx.Err() != nil || !t.homeVerified() {
		return data
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(data, &envelope) != nil {
		return data
	}
	id, ok := trustID(envelope["id"])
	if !ok {
		return data
	}
	t.mu.Lock()
	entry, tracked := t.pending[id]
	delete(t.pending, id)
	t.mu.Unlock()
	if !tracked || t.now().Sub(entry.at) > 5*time.Second {
		return data
	}
	if e := envelope["error"]; len(e) > 0 && string(e) != "null" {
		return data
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(envelope["result"], &result) != nil {
		return data
	}
	var layers []struct {
		Name struct {
			Type string `json:"type"`
		} `json:"name"`
		Disabled json.RawMessage `json:"disabledReason"`
	}
	rawLayers := bytes.TrimSpace(result["layers"])
	if len(rawLayers) == 0 || rawLayers[0] != '[' || json.Unmarshal(rawLayers, &layers) != nil {
		return data
	}
	for _, layer := range layers {
		if d := bytes.TrimSpace(layer.Disabled); len(d) > 0 && string(d) != "null" {
			return data
		}
		switch layer.Name.Type {
		case "packagedDefaults", "mdm", "system", "enterpriseManaged", "user", "sessionFlags", "legacyManagedConfigTomlFromFile", "legacyManagedConfigTomlFromMdm":
		default:
			return data
		}
	}
	var config map[string]json.RawMessage
	if json.Unmarshal(result["config"], &config) != nil {
		return data
	}
	var projects map[string]json.RawMessage
	if json.Unmarshal(config["projects"], &projects) != nil || projects == nil {
		return data
	}
	// At most one filesystem proof can remain outstanding on this connection.
	// A stalled read cannot grow a goroutine per config reply or authorize late.
	select {
	case t.proofSlot <- struct{}{}:
	default:
		return data
	}
	ctx, cancel := context.WithTimeout(t.ctx, trustConfigBudget)
	defer cancel()
	type proofResult struct {
		proof worktreeTrustProof
		ok    bool
	}
	ready := make(chan proofResult, 1)
	reader := t.read
	go func() {
		defer func() { <-t.proofSlot }()
		before, ok := reader(ctx, entry.cwd)
		if !ok || ctx.Err() != nil {
			ready <- proofResult{}
			return
		}
		after, ok := reader(ctx, entry.cwd)
		ready <- proofResult{after, ok && before == after}
	}()
	var proof worktreeTrustProof
	select {
	case <-ctx.Done():
		return data
	case got := <-ready:
		if !got.ok || ctx.Err() != nil {
			return data
		}
		proof = got.proof
	}
	if !trustAliasAllowed(entry.cwd, proof, projects) {
		return data
	}
	projects[entry.cwd] = json.RawMessage(`{"trust_level":"trusted"}`)
	raw, err := json.Marshal(projects)
	if err != nil {
		return data
	}
	config["projects"] = raw
	raw, err = json.Marshal(config)
	if err != nil {
		return data
	}
	result["config"] = raw
	raw, err = json.Marshal(result)
	if err != nil {
		return data
	}
	envelope["result"] = raw
	raw, err = json.Marshal(envelope)
	if err != nil || ctx.Err() != nil || !t.homeVerified() {
		return data
	}
	return raw
}

func trustAliasAllowed(cwd string, p worktreeTrustProof, projects map[string]json.RawMessage) bool {
	if p.Fingerprint == "" || p.Cwd == "" || p.MainRoot == "" || p.Checkout == "" {
		return false
	}
	level := func(raw json.RawMessage) string {
		var v struct {
			Level string `json:"trust_level"`
		}
		if json.Unmarshal(raw, &v) != nil {
			return ""
		}
		return v.Level
	}
	trusted := level(projects[p.MainRoot]) == "trusted" || level(projects[p.OriginalMain]) == "trusted"
	if !trusted {
		return false
	}
	for _, path := range []string{cwd, p.Cwd, p.Checkout} {
		if _, exists := projects[path]; exists {
			return false
		}
	}
	for path, raw := range projects {
		if !filepath.IsAbs(path) || level(raw) != "untrusted" {
			continue
		}
		for _, target := range []string{cwd, p.Cwd, p.Checkout, p.MainRoot, p.OriginalMain} {
			rel, err := filepath.Rel(filepath.Clean(path), filepath.Clean(target))
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return false
			}
		}
	}
	return true
}
