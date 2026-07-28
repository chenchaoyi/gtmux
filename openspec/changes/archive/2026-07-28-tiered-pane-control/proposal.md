# tiered-pane-control

## Why

gtmux's rich surface is agent-only: the radar lists coding-agent panes, and only
those get view/jump/send/dispatch/HQ. But the control primitives underneath —
`gtmux focus <pane>`, `gtmux send <pane>`, `gtmux attach <host> %pane` — are already
**pane-level**: they act on any tmux pane by id, agent or not. So a whole spectrum
is reachable in principle but has no first-class surface:

- **agent pane in tmux** — full intelligence (digest, 1/2/3 approval, dispatch, HQ). ✅
- **plain tmux pane** (a dev server, a log tail, a shell next to the agent) — you can
  already `focus`/`send`/`attach` it, but there's no way to SEE or reach it from
  gtmux; you have to know its id.
- **non-tmux agent** — sensed read-only (no pane to drive). ✅

The gap showed up concretely: when an agent exits, its tmux pane persists as a plain
shell, and gtmux had no honest way to represent or reach it (a follow-up already
makes `focus` say "the agent here has exited"). More broadly, users hit dead-ends —
"my agent's dev-server pane is right there but gtmux can't show it."

The tension is real and must be respected: **the agent radar is gtmux's moat.** If we
flatten every pane (vim, htop, logs) into the radar, "who needs you" is drowned and
gtmux degrades into yet another tmux session list — an undifferentiated category tmux
itself already fills. So this change is a **surfacing + tiering** design, not a
capability rearchitecture, and its first constraint is: *do not dilute the radar.*

## What Changes

Introduce a **tiered pane-control model** across three surfaces, keeping the radar
agent-first and putting general pane management on a separate, opt-in surface:

- A **sessions/panes browser** — a SEPARATE secondary surface (not the radar) that
  lists all tmux sessions → windows → panes and lets you focus/type/attach any of
  them. This is where "manage all of tmux" lives, so the radar stays clean. Backed by
  a new read-only CLI producer (`gtmux panes --json`) the menu-bar/phone/web consume.
- **Agent-neighbor panes** — inside an agent's Detail, show and switch to the sibling
  panes in the same window/session (the shell next to the agent). Scoped, natural, no
  radar pollution.
- **Opt-in "watch this pane"** — deliberately promote a chosen non-agent pane (a dev
  server, a log) into the radar. User-chosen, never automatic; clearly a watched-pane
  row, not an agent row.
- **A tiered capability contract** — the same primitives, different power by tier:
  agent panes get the full intelligence; plain panes get focus + type + watch +
  attach; sensed non-tmux agents stay read-only. Documented so every surface applies
  it consistently.

Brand framing (positioning guard, not code): "**agents first, but you're never stuck —
any pane is reachable.**" gtmux stays a coding-agent command center; "reach any pane"
is a capability that removes dead-ends, NOT a repositioning to a generic tmux manager.

Out of scope (explicitly deferred): rich TUI preview for arbitrary panes on mobile,
auto-classifying non-agent panes into meaningful states, and any change to how agent
panes are detected or ranked.

## Capabilities

### New Capabilities
- `pane-browser`: a read-only enumeration of all tmux sessions/windows/panes
  (`gtmux panes --json`) and the secondary browser surface built on it, plus the
  tiered control contract (what each pane tier may do) and the opt-in "watch this
  pane" promotion. Kept separate from the agent radar by construction.

### Modified Capabilities
- `agent-radar`: the radar gains an opt-in "watched" pane row type (a promoted
  non-agent pane) that is visually and semantically distinct from agent rows and never
  auto-added — a requirement about what MAY appear on the radar and how it's marked.
- `terminal-jump`: `gtmux focus`/`send` gain an explicit, documented contract for
  NON-agent panes (they already work on any pane id) — a requirement pinning that
  general panes are first-class jump/type targets with the tier's capabilities.

## Impact

- New CLI surface: `gtmux panes [--json]` (read-only producer in `internal/radar`,
  the pane-data kernel). Documented in CLAUDE.md command list, `gtmux --help`,
  `docs/cli.md`, and `api/contract.md` if it gets an HTTP endpoint.
- Menu-bar app: a new sessions/panes browser view + agent-neighbor panes in Detail +
  a "watch this pane" affordance (Swift; DESIGN.md gets a new section — kept distinct
  from the radar section so clarity is preserved).
- Mobile/web: the browser + neighbor panes generalize the existing surfaces (MOBILE.md
  / WEB.md notes), reusing the same `panes --json` + `send`/`attach` contract.
- No change to agent detection, ranking, digest, dispatch, or HQ. The radar's agent
  rows are untouched except for the additive, opt-in watched-pane row type.
