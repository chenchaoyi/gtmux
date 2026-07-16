# Design: pair-share-model

## Context

Credentials already have per-token identity and scope (`EnrolledDevice.Scope`:
`""` = owner device, `"guest"` = share link; `auth()` resolves master/device/guest
into the request context) — but guest read/input checks consult ONE global
`ShareState` (enabled + panes + view_panes), discarding which guest is asking.
The pairing flows are per-surface one-offs; the terminal has none.

## Goals / Non-Goals

**Goals:**
- Per-link guest scope (view/input sets + optional expiry) enforced server-side.
- Zero contract break: `GET /api/share` keeps `{input, all, panes, view_panes}` —
  it was always "the CALLER's capability"; guests now resolve from their token.
- Upgrade preserves behavior exactly (migration + template/broadcast semantics)
  until the new UIs land.
- One pairing story, three media (QR / URL+code / attach one-liner); the terminal
  becomes a first-class owner surface.
- Pair/Share vocabulary consistent across CLI, menu bar, app, web.

**Non-Goals:**
- One-time-use share links; per-link rate limits (later if ever).
- Changing the consent master switch (`share on/off`) — it stays the single
  host-level gate for ALL guest typing.
- Multi-host federation; share links for HQ/digest surfaces (guests stay barred).

## Decisions

1. **Scope lives on the token** (`EnrolledDevice`): guest entries gain
   `ViewPanes []string`, `InputPanes []string`, `ExpiresAt int64` (0 = never).
   Owner devices keep nil/0 (unrestricted). Input ⊆ view is normalized at every
   write (union input into view).
2. **Auth carries the caller.** `auth()` stashes the resolved `EnrolledDevice`
   (guests) in the request context beside the scope. Guest checks become
   `CanViewFor(dev, pane)` / `AllowedFor(dev, pane)` on the ShareManager
   (consent stays global). Expired guests (`ExpiresAt` past) fail auth with 401
   exactly like a revoked token; expiry is checked in `TokenScope`.
3. **Migration (commander decision a):** on serve start, any guest entry with
   nil scope fields gets a ONE-TIME copy of the legacy global lists
   (`migrated:true` semantics via non-nil-but-possibly-empty slices thereafter).
   The global lists remain in `share.json` with two ongoing roles:
   - **template**: `share new` without explicit flags copies the current global
     lists (preserves the "mint, then tick" flow's semantics);
   - **broadcast**: the legacy global mutations (`share add/remove`,
     `share view add/remove`, `POST /api/share` config) apply to the template AND
     fan out to every existing guest link — byte-for-byte the old behavior as
     observed by guests, until S3's per-link UI replaces the habit.
4. **Per-link CLI**: `gtmux share new --label X [--view %1,%2] [--type %1]
   [--expires 24h|7d|never]`; `gtmux share set <id> [--view …] [--type …]
   [--expires …]` (replace semantics per flag; omitted flag untouched);
   `share status` lists per-link scope summaries (`2 view · 1 type · expires 3h`);
   `--json` gains per-guest `view_panes`, `panes`, `expires_at` (additive).
5. **`gtmux pair`** (new command; `gtmux devices` aliased): `pair` mints one
   enroll code and renders it three ways (QR block for the phone, `https://…/#c=`
   URL for a browser, `gtmux attach '<url>#c=<code>'` one-liner for a terminal);
   `pair list` = roster of owner devices (guests live under `share status`);
   `pair revoke <id>`. Implementation reuses the existing mint/enroll endpoints —
   no new server surface.
6. **Attach pair flow**: an attach target whose fragment carries `c=<code>`
   redeems it via `POST /api/enroll` (name = local hostname), persists
   `{host → token}` in `~/.config/gtmux/remotes.json` (0600), and proceeds as an
   owner. A bare `gtmux attach <host>` first consults remotes.json, then
   `--token`. Guest links (`#t=`) unchanged.
7. **Menu-bar S3**: Preferences sections become 远程访问 / 你的设备 Pair /
   分享 Share. Pair: device rows (icon by kind guessed from name/UA, last seen,
   revoke) + one "配对新设备" sheet with the three media (reusing Pairing.swift's
   QR). Share: consent switch + per-link rows (label · scope summary · created ·
   revoke) with an inline expandable See/Type session picker (per link — the
   #457 columns move here) + "新建分享" sheet (name + scope in one step).
   ShareStore drives everything through the CLI (`share new/set/revoke`), never
   the token roster.
8. **S4 vocabulary**: app server list sections "我的 Mac (配对)" / "访客连接
   (分享)"; guest banner shows granted scope counts; web guest page adds a
   "your access: N viewable · M typable" strip from `GET /api/share`; all
   user-facing copy says 配对/Pair for self, 分享/Share for collaborators.

## Risks / Trade-offs

- **Broadcast semantics could surprise** once per-link editing exists (S3): a
  global `share add` overwrites per-link tailoring. Mitigation: S3's UI edits
  per-link only; the CLI global forms print a "fans out to N links" notice; docs
  mark them legacy.
- **share.json + roster consistency**: scope now spans two files (consent in
  share.json, per-link lists in the roster). Single-writer serve owns both;
  the CLI mutates via the API/CLI only.
- **Expiry check cost**: TokenScope already runs per request; comparing an int64
  is free.
- **attach storing owner tokens on another machine** widens credential surface —
  file is 0600, documented, and `pair revoke` kills it server-side instantly.

## Migration

- Serve start: guest entries lacking scope fields ← copy of legacy global lists.
- `POST /api/share/new` accepts optional `view/input/expires` (additive).
- Older CLIs against a newer serve keep working (global forms preserved);
  newer CLI flags against an older serve fail with a clear "serve too old".
