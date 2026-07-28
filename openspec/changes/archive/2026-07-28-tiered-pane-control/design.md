## Context

The control primitives are already pane-general (`focus`/`send`/`attach` act on any
pane id); only the DISCOVERY layer (`internal/radar`, `classifyAgent`) is agent-only,
excluding bare shells (`!isAgent → continue`). The radar's agent-awareness is the
moat. So the design question is not "can we control any pane" (we can) but "how do we
SURFACE that without turning the radar into a generic tmux session list."

Prior deliberation (2026-07-10, memory `scope-tmux-ssh-allwindows`) already reached
the shape used here: don't flatten all windows into the radar; do agent-neighbor
panes + a SEPARATE sessions browser + opt-in watch-this-pane. This change formalizes
and specs that.

## Goals / Non-Goals

**Goals:**
- Any tmux pane is reachable and typeable from a gtmux surface, not just agent panes.
- The agent radar stays agent-first and visually clean — no automatic non-agent rows.
- One shared data contract (`gtmux panes --json`) so menu-bar/phone/web build the same
  browser without each re-scanning tmux.
- A written **tier contract**: what each pane tier (agent / plain tmux / sensed
  non-tmux) may do, applied consistently across surfaces.
- Positioning unchanged: coding-agent command center; "reach any pane" is a capability,
  not the headline.

**Non-Goals:**
- No auto-promotion of non-agent panes into the radar (opt-in only).
- No attempt to classify a non-agent pane's "state" (a dev server isn't waiting/idle).
- No rich live-preview of arbitrary panes on mobile in this change (capture-pane view
  can follow; not required for reach/type).
- No change to agent detection, ranking, digest, dispatch, or HQ.

## Decisions

**1. A separate `pane-browser` capability, not a radar extension.**
General "all of tmux" management is its own surface and its own spec capability. The
radar spec is touched only additively (the opt-in watched row). This is the core
anti-dilution decision: two capabilities, two surfaces, one clear boundary.

**2. `gtmux panes --json` — a new read-only producer in `internal/radar`.**
Enumerates sessions → windows → panes with the fields a browser needs: pane id, loc,
session/window/pane indices, cwd, current command, title, active flag, in-copy-mode,
and a `tier` field (`agent` | `plain`) computed by reusing `classifyAgent` (agent panes
are cross-referenced to the radar so the browser can badge them and link to their
Detail). Read-only; no side effects; lives in the pane-data kernel next to
`GatherAgents`. `--json` is the machine contract; a plain-text tree is the human view.
Rationale for a distinct command (vs. extending `agents --json`): `agents --json` is a
locked contract meaning "coding agents"; overloading it with shells would break that
promise and every consumer's assumptions.

**3. Tier contract (the capability matrix), enforced by surface, not by the core.**
The primitives don't gate on tier (they're pane-level); each SURFACE applies the tier
to decide what to OFFER:

| tier | discover | view (capture) | focus/jump | type/send | attach | intelligence (digest/1·2·3/dispatch/HQ) |
|---|---|---|---|---|---|---|
| agent (tmux) | radar (auto) | ✅ | ✅ | ✅ | ✅ | ✅ |
| plain tmux pane | pane-browser (opt-in) | ✅ | ✅ | ✅ | ✅ | ✕ (no agent to reason about) |
| sensed non-tmux agent | radar "Elsewhere" | ✕ | ✕ | ✕ | ✕ | ✕ (read-only) |

Guest scope still applies on top (a shared pane's view/type allowlist is unchanged).

**4. Agent-neighbor panes in Detail.**
An agent's Detail gains a compact "in this window/session" strip listing sibling panes
(from `panes --json` filtered to the agent's session) with focus/type. Scoped to the
agent's own neighborhood — the common real need ("the shell next to my agent") — with
zero radar impact.

**5. Opt-in "watch this pane" → an additive radar row TYPE.**
A user promotes a chosen plain pane; it appears on the radar as a distinct **watched**
row (own glyph/label, e.g. an "eye", NOT an agent status), persisted in a small
`watched/` marker set keyed by pane id. It never carries waiting/working/idle (those
are agent concepts); it's "a pane you asked to keep an eye on." Removed as easily as
added, and auto-dropped when its pane closes (reusing the orphan-marker sweep).

**6. Brand/positioning is a guard, enforced in docs/design, not code.**
DESIGN.md gets a NEW section for the pane browser, deliberately separate from the radar
section, stating the anti-dilution rule so future iterations don't merge them. Help
text and docs frame plain-pane reach as "you're never stuck," not "tmux manager."

## Risks / Trade-offs

- **Radar dilution (the main risk).** Mitigated structurally: non-agent panes live on a
  separate surface; the only radar addition is opt-in and visually distinct. If a future
  change tries to auto-add panes to the radar, the spec forbids it.
- **Scope creep toward "tmux GUI."** Bounded by Non-Goals: no state classification, no
  arbitrary live preview this round. The browser is reach + type, not a terminal.
- **A second pane enumeration cost.** `panes --json` is only invoked when a browser is
  open (not on the radar's 1.5s poll), so it's off the hot path.
- **`panes --json` vs `agents --json` confusion.** Mitigated by clear docs and the
  `tier` field: `agents` = coding agents (locked), `panes` = everything (new).
