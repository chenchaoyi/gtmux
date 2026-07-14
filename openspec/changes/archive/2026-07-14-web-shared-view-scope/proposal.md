## Why

Web-shared input gates **typing** per pane, but **viewing is wide open**: any guest
holding a share link can read the host's ENTIRE radar (`GET /api/agents`) and any
pane's live screen (`GET /api/pane`, the SSE stream, the web mirror) — only
`POST /api/send` is scoped. So "share one pane" today silently exposes every
session. This change makes VIEW a first-class, host-controlled scope, separate
from input, so a guest sees only what the host chose to show.

## What Changes

- Add a second per-pane allowlist — **view scope** (`viewPanes`) — alongside the
  existing **input scope** (`panes`), persisted separately in the share state.
- Invariant **input ⊆ view**: an input-allowed pane is implicitly view-allowed;
  removing a pane's view removes its input too.
- **BREAKING (guest-visible behavior):** default view scope is now **secure** — a
  guest sees NOTHING until the host allows a pane for viewing (previously a guest
  saw everything). Consent OFF still means view-only-nothing + no input.
- Server: for **guest scope only**, every read surface is filtered by `viewPanes`
  — `/api/agents` drops non-viewable rows, `/api/pane` refuses non-viewable panes,
  the SSE agent stream and the web mirror are filtered likewise. master/device
  scopes are unaffected (full view). The server gate stays authoritative.
- CLI: `gtmux share` gains view commands (`share view on|off|add %N|remove %N`);
  `share status` shows both allowlists; `share add %N` (input) implies view.
- Menu-bar: the Shared-input pane picker (redesigned in #427) gets **two controls
  per row** — 👁 can-see and ⌨️ can-type — with the type control disabled unless
  see is on. Same AgentAvatar + session identity per row.
- Web (guest page): renders only viewable panes; the input row still shows only on
  input-allowed panes (already true).

### Non-goals

- No change to **master/device** scope — the owner and their paired phone keep
  full view + input. This is strictly about the **guest** (share-link) scope.
- Not per-field redaction or a "blur screen" mode — a pane is either viewable
  (full read) or not present. Granularity stays per-pane, matching input.
- Read-only invariant unchanged: viewing never mutates the pane.

## Capabilities

### New Capabilities
<!-- none — extends the existing remote-access share model -->

### Modified Capabilities
- `remote-access`: the guest share model gains a per-pane VIEW allowlist separate
  from input, the input⊆view invariant, secure-by-default view scope, and
  guest-scope filtering of all read endpoints (agents/pane/SSE/mirror).
- `menu-bar-app`: the Shared-input allowlist row exposes two independent controls
  (view / input) instead of a single input checkbox.

## Impact

- **Server (Go):** `internal/server/share.go` (ShareManager: `viewPanes`, invariant,
  `CanView`), `internal/server/server.go` (`handleAgents`/`handlePane` guest-scope
  filtering), the SSE agents event, and the share.json persisted state shape.
- **CLI (Go):** `gtmux share` subcommands + `internal/i18n` strings + `docs/cli.md`.
- **Menu-bar (Swift):** `Preferences.swift` (two-control rows), `ShareStore.swift`
  (view state + mutations), reuses `AgentStore.shareablePanes` / `AgentAvatar`.
- **Web:** guest mirror filters panes by view scope.
- **Contracts:** additive to share.json + `gtmux share --json`; the guest-visible
  `/api/agents` / `/api/pane` responses become filtered (behavior change for guest
  tokens only). Tests: server view-filter + invariant; menu-bar two-control rows.
