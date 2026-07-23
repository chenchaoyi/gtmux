// Package driver is the per-agent driver registry — the Layer-2 half of the
// two-layer perception/drive model (openspec change agent-drivers). A Driver is a
// set of OPTIONAL capabilities an agent's structured interfaces can provide
// (delivery receipt from its hook event stream, state truth, transcript content,
// readiness, headless one-shot); a nil capability means that channel falls back to
// Layer 1 — the tmux screen/keystroke base, which is permanently retained and
// behaviorally identical to the pre-driver system.
//
// Drivers consume facts the agent already produces (events.jsonl, transcript
// files, state markers, exec output). They never wrap the agent in a proxy or a
// persistent session: the tmux pane stays the single input path, so the user can
// always jump in and take over.
//
// This package is a LEAF (only usercfg below it) so every layer — dispatch,
// dispatchbridge, radar, hqnudge, app — may consult it without an import cycle.
package driver

import "github.com/chenchaoyi/gtmux/internal/usercfg"

// Verdict is the three-valued outcome of a Receipt check. Positive evidence is
// monotonic (invariant I2): Confirmed is final and cannot be overturned by a
// screen read; NoEvidence is NOT failure — it only defers to Layer 1.
type Verdict int

const (
	// NoEvidence — the driver has nothing to say; the channel falls to Layer 1.
	NoEvidence Verdict = iota
	// Confirmed — the event stream proves the payload was submitted. Final.
	Confirmed
	// Unsubmitted — the stream proves the paste is in place but was never
	// submitted (the precise "swallowed Enter"): repair is Enter-only.
	Unsubmitted
)

// Driver is one agent's registered capability set. Every capability field may be
// nil — nil means "no structured interface for this channel, use Layer 1".
type Driver struct {
	// Name is the agent key — the same naming domain as `gtmux hook --agent`
	// and the radar profiles ("claude", "codex", …).
	Name string

	// HookEquipped records that this agent installs gtmux hooks, so its prompt
	// submissions land on the session-events stream. This is BASELINE fact, not
	// a driver capability: the pre-driver system already prefers the event
	// stream for these agents (dispatch verify), so it is deliberately NOT
	// subject to the capability switches — disabling drivers must restore the
	// Layer-1-only path only for behaviors the driver layer itself added.
	// From P1 the hook-first verify collapses into Receipt and this field's
	// consumers migrate to `Receipt != nil`.
	HookEquipped bool

	// Receipt confirms a delivery from the agent's event stream: needle is the
	// shared-normalization head of the delivered payload, since bounds the
	// event window (unix seconds). Nil until the receipt capability ships (P1).
	Receipt func(pane, needle string, since int64) Verdict
}

// registry holds the built-in drivers, keyed by agent key. P0 collapses the
// former dispatchbridge hookAgents whitelist here; capabilities arrive per
// phase (Receipt in P1, readiness in P3, headless in P5).
var registry = map[string]Driver{
	"claude":       {Name: "claude", HookEquipped: true},
	"codex":        {Name: "codex", HookEquipped: true},
	"gemini":       {Name: "gemini", HookEquipped: true},
	"cursor":       {Name: "cursor", HookEquipped: true},
	"cursor-agent": {Name: "cursor-agent", HookEquipped: true},
	"opencode":     {Name: "opencode", HookEquipped: true},
	"copilot":      {Name: "copilot", HookEquipped: true},
	"kiro":         {Name: "kiro", HookEquipped: true},
}

// For resolves the driver for an agent key. An unknown agent yields the zero
// Driver (all capabilities nil → Layer 1 everywhere). Capability switches from
// the user config (`driver.enable`, `driver.<agent>.<capability>`) strip the
// corresponding capability functions; they never touch HookEquipped (see the
// field comment — baseline fact, not a driver behavior).
func For(agentKey string) Driver {
	d, ok := registry[agentKey]
	if !ok {
		return Driver{Name: agentKey}
	}
	sw := loadSwitches()
	if !sw.enabled() || !sw.capOn(agentKey, "receipt") {
		d.Receipt = nil
	}
	return d
}

// switches is the parsed `driver` object of ~/.config/gtmux/config.json:
//
//	"driver": {"enable": false, "claude": {"receipt": false}}
//
// Every knob defaults to ON; only an explicit false turns a capability off.
type switches struct {
	Enable *bool `json:"enable"`
	// Agents maps agent key → capability name → enabled.
	Agents map[string]map[string]*bool `json:"-"`
}

func (s switches) enabled() bool { return s.Enable == nil || *s.Enable }

func (s switches) capOn(agent, capability string) bool {
	v, ok := s.Agents[agent][capability]
	return !ok || v == nil || *v
}

// loadSwitches reads the driver switches; a missing/malformed config yields
// all-on defaults (the config can only turn things OFF).
func loadSwitches() switches {
	var c struct {
		Driver map[string]any `json:"driver"`
	}
	var s switches
	if usercfg.Load(&c) != nil || c.Driver == nil {
		return s
	}
	if v, ok := c.Driver["enable"].(bool); ok {
		s.Enable = &v
	}
	s.Agents = map[string]map[string]*bool{}
	for agent, raw := range c.Driver {
		caps, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		m := map[string]*bool{}
		for capName, val := range caps {
			if b, ok := val.(bool); ok {
				v := b
				m[capName] = &v
			}
		}
		s.Agents[agent] = m
	}
	return s
}
