## Context

Web-shared input (shipped #418) added a `guest` token scope whose `POST /api/send` is
gated by a consent toggle + a per-pane input allowlist (`internal/server/share.go`
`ShareManager`). But the READ surface was never scoped: `handleAgents` / `handlePane`
(`internal/server/server.go`), the SSE agent stream, and the web mirror return the full
radar and any pane's text to ANY authenticated token, including guests. A guest link
therefore leaks the whole workspace; only typing is contained. This change adds a VIEW
scope symmetric to input and filters all guest reads by it.

## Goals / Non-Goals

**Goals:**
- A guest sees only host-chosen panes; a fresh guest sees nothing (secure default).
- View and input are independent per-pane allowlists, with the invariant input ⊆ view.
- The gate is server-side and authoritative; CLI + menu-bar only drive it.
- Additive contract: master/device behaviour and the `agents --json` shape are unchanged.

**Non-Goals:**
- No change to master/device (owner) scope — full view + input as today.
- No sub-pane redaction / blur — a pane is fully viewable or absent (per-pane grain).
- No new capability spec — this extends `remote-access` + `menu-bar-app`.

## Decisions

- **Second allowlist in ShareManager, not a new token type.** Keep the existing scope
  model (`full`/`guest`); add `viewPanes map[string]bool` beside `panes`, persisted in
  `share.json` as `view_panes`. `CanView(scope, pane)` returns true for `full`, else
  `viewPanes[pane]`. `Allowed` (input) additionally requires consent + `panes[pane]`.
  Alternative (a distinct "view token") rejected: it doubles the roster and the URL
  scheme for no gain — one guest link with two allowlists is simpler.
- **Enforce input ⊆ view in SetConfig.** Adding a pane to `panes` also adds it to
  `viewPanes`; removing it from `viewPanes` also removes it from `panes`. The invariant
  lives in ONE place (the mutator) so no caller can violate it. `gtmux share add %N`
  (input) implies view by construction.
- **Filter reads at the handler, keyed off request-context scope.** `auth()` already
  carries the caller scope in context. `handleAgents` decodes the agents JSON and drops
  non-viewable rows for guest scope; `handlePane` returns `403` for a non-viewable pane;
  the SSE agents event and the web mirror reuse the same `CanView` filter. `full` scope
  skips filtering entirely (fast path, byte-identical output). Filtering the JSON array
  (rather than plumbing scope into the producer `AgentsJSON()`) keeps the CLI producer
  scope-agnostic and the contract stable.
- **Secure default (B), called out as BREAKING for guests.** Empty `viewPanes` ⇒ guest
  sees nothing. Existing share.json without `view_panes` decodes to empty, so on upgrade a
  live guest link goes from "sees all" to "sees nothing" until the host adds view panes —
  the safe direction (less exposure). Documented in the proposal + release notes.
- **Menu-bar: two checkboxes per existing picker row.** Reuse `shareablePanes` +
  `AgentAvatar` from #427; add a 👁 and a ⌨️ toggle bound to view/input, ⌨️ disabled
  unless 👁 is on. `ShareStore` gains `viewPanes` + `setView(pane,on)` calling
  `gtmux share view add/remove`.

## Risks / Trade-offs

- [A live guest silently loses visibility on upgrade] → Intended (secure default); call it
  out in release notes and `gtmux share status` shows the now-empty view list so the host
  re-grants deliberately.
- [Read filtering missed on one surface = leak] → Enumerate every guest-reachable read
  (`/api/agents`, `/api/pane`, SSE agents event, web mirror, and check `/api/usage` +
  `/api/digest`) and add a test per surface asserting a guest sees only view panes.
- [SSE stream must re-filter on every push, per subscriber scope] → Filter in the event
  serializer using the subscriber's scope; guests already get their own filtered snapshot
  on connect, so the incremental path uses the same filter.
- [`/api/usage` and `/api/digest` also leak fleet data to guests] → Decide in
  implementation: simplest is to refuse both for guest scope (they are owner/HQ surfaces,
  not part of the shared view). Tracked in Open Questions.

## Open Questions

- Should a guest reach `/api/usage` and `/api/digest` at all? Proposed: **no** — refuse
  for guest scope (they expose the whole fleet + token budgets). Confirm during apply.
- View grain: per-pane (chosen, matches input) vs per-session (allow a whole tmux
  session at once). Per-pane ships first; a "whole session" convenience toggle can layer
  on later without a contract change.
