// Package driver is the per-agent driver registry — the Layer-2 half of the
// two-layer perception/drive model (openspec change agent-drivers). A Driver is a
// set of OPTIONAL capabilities an agent's structured interfaces can provide
// (delivery receipt from its hook event stream, state truth, transcript content,
// readiness, headless one-shot); a nil capability means that channel falls back to
// Layer 1 — the tmux screen/keystroke base, which is permanently retained.
//
// Drivers consume facts the agent already produces (events.jsonl, transcript
// files, state markers, exec output). They never wrap the agent in a proxy or a
// persistent session: the tmux pane stays the single input path, so the user can
// always jump in and take over.
//
// Import direction: this package sits BELOW dispatchbridge/radar/hqnudge/app and
// ABOVE the evidence leaves it reads (events, dispatch's pure matching helpers) —
// dispatch never imports driver (its event evidence is injected via dispatch.IO),
// so the layering stays acyclic.
package driver

import (
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

// Verdict is the three-valued outcome of a Receipt check. Positive evidence is
// monotonic (invariant I2): Confirmed is final and cannot be overturned by a
// screen read; NoEvidence is NOT failure — it only defers to Layer 1.
type Verdict int

const (
	// NoEvidence — the driver has nothing to say; the channel falls to Layer 1.
	NoEvidence Verdict = iota
	// Confirmed — the event stream proves the payload was submitted. Final.
	Confirmed
	// Unsubmitted — the evidence proves the paste is in place but was never
	// submitted (the precise "swallowed Enter"): repair is Enter-only.
	Unsubmitted
)

// Driver is one agent's registered capability set. Every capability field may be
// nil — nil means "no structured interface for this channel, use Layer 1".
type Driver struct {
	// Name is the agent key — the same naming domain as `gtmux hook --agent`
	// and the radar profiles ("claude", "codex", …).
	Name string

	// Receipt confirms a delivery from the agent's session-events stream: needle
	// is the dispatch.NormalizeNeedle fingerprint of the delivered payload, since
	// bounds the event window (unix seconds). Non-nil for every hook-equipped
	// agent — `Receipt != nil` is the fact the event-first verify path keys on
	// (it replaced the P0 HookEquipped field), so the capability switches can
	// force the pure Layer-1 screen path (design §5).
	Receipt func(pane, needle string, since int64) Verdict

	// Ready reports the deterministic "session came up" signal for a pane: has a
	// session-start event appeared for it since the launch moment (unix seconds)?
	// Consumers use it to SHORT-CIRCUIT the spawn ready gate's two-frame settle
	// wait (one input-ready capture then suffices). Positive-only (I2): false is
	// never failure evidence — the full screen gate applies unchanged.
	Ready func(pane string, since int64) bool

	// Content loads a session's structured conversation turns — the transcript
	// parser behind the digest's goal/last. Registered only where a parser
	// exists (claude, codex); nil elsewhere, and the digest row renders from
	// radar signals alone (the design rule: every field degrades to "").
	Content func(sessionID string, maxTurns int) ([]transcript.Turn, error)
}

// hookEquippedAgents are the agents whose installers wire gtmux hooks, so their
// prompt submissions land on the session-events stream — the former
// dispatchbridge whitelist. They all share the SAME events-backed Receipt: the
// evidence is the stream record gtmux's own hook writes, not anything
// agent-specific, so registering one is registering all (the commander's P1
// ruling covered the sparse-events case explicitly: Codex ships in the same
// batch as Claude — low event density only lowers the hit rate, and NoEvidence
// falls to Layer 1).
var hookEquippedAgents = []string{
	"claude", "codex", "gemini", "cursor", "cursor-agent", "opencode", "copilot", "kiro",
}

// registry holds the built-in drivers, keyed by agent key. Further capabilities
// arrive per phase (content wiring in P4, headless in P5). Ready, like Receipt,
// is shared by every hook-equipped agent: the hook NORMALIZES each agent's
// session-start-ish raw event (SessionStart / session_start / on_session_start /
// agentSpawn, …) to one `SessionStart` stream record, so the evidence is
// agent-agnostic — an agent whose hook never emits it simply never
// short-circuits, which changes nothing (I2).
var registry = func() map[string]Driver {
	m := make(map[string]Driver, len(hookEquippedAgents))
	for _, k := range hookEquippedAgents {
		m[k] = Driver{Name: k, Receipt: eventsReceipt, Ready: eventsReady}
	}
	// Content only where a transcript parser exists (internal/transcript's
	// per-agent log readers) — a pure re-wiring of today's transcript.Load.
	for _, k := range []string{"claude", "codex"} {
		d := m[k]
		key := k
		d.Content = func(sessionID string, maxTurns int) ([]transcript.Turn, error) {
			return transcript.Load(key, sessionID, maxTurns)
		}
		m[k] = d
	}
	return m
}()

// For resolves the driver for an agent key. An unknown agent yields the zero
// Driver (all capabilities nil → Layer 1 everywhere). Capability switches from
// the user config (`driver.enable`, `driver.<agent>.<capability>`) strip the
// corresponding capability functions — a stripped Receipt means delivery
// verification runs the pure Layer-1 screen path, deliberately MORE conservative
// than the default, for isolating event-channel faults (design §5).
func For(agentKey string) Driver {
	d, ok := registry[agentKey]
	if !ok {
		return Driver{Name: agentKey}
	}
	sw := loadSwitches()
	if !sw.enabled() || !sw.capOn(agentKey, "receipt") {
		d.Receipt = nil
	}
	if !sw.enabled() || !sw.capOn(agentKey, "ready") {
		d.Ready = nil
	}
	if !sw.enabled() || !sw.capOn(agentKey, "content") {
		d.Content = nil
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
