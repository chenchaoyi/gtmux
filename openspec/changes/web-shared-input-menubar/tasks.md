# Tasks — web-shared input: menu-bar control surface

## 1 — CLI `--json` (the app's contract)
- [x] `gtmux share status --json` → `{enabled, panes, guests:[{id,label,enrolled_at}], base}` (token-free)
- [x] `gtmux share new [--label] --json` → `{id, label, url}` (no bare token; url carries `#t=`)
- [x] Pure builders (`buildShareStatus`, `buildShareNew`) so the shape is unit-tested without a live serve
- [x] Go test: builders map fields correctly (guest name→label, enrolled_at, url assembly); live-verified

## 2 — `ShareStore` (Swift, mirrors `RemoteAccess`)
- [x] `@Published enabled / allowedPanes / guests / base / busy / lastError / lastMintedLink`
- [x] `refresh()` reads `~/.config/gtmux/share.json` directly (cheap, poll-safe, no secrets) → enabled + allowedPanes
- [x] `loadDetail()` shells `gtmux share status --json` (on section-appear + after a mutation) → guests + base
- [x] mutations shell out off-main: `setEnabled`, `setPane(_,allowed:)`, `revoke(id)`; `newLink(label:)` → copies URL
- [x] `parseStatus(Data)` a pure static parser → Swift test decodes the `--json` shape

## 3 — Preferences "Shared input" section
- [x] consent Toggle (default reflects state; disabled while busy)
- [x] per-pane allowlist from `AgentStore` (tmux panes only: `!isNative && paneID != ""`), each a checkbox
- [x] guest links list (label + "ago") each with Revoke; "New link" button (mints + copies, shows the link)
- [x] a live-exposure subtitle when consent on + panes + guests (exposure is never silent)
- [x] en/zh strings via `l10n.tr`; plain copy (no marketing tone)

## 4 — Popover footer exposure indicator
- [x] compact icon-only glyph when shared input is LIVE (enabled && panes && guests), `.fixedSize()` so it can't clip the version
- [x] tap opens Preferences (same as the "Remote on" indicator)

## 5 — Specs + gates
- [x] `menu-bar-app` spec: ADD "Shared-input control surface" requirement + scenarios
- [x] `remote-access` spec: MODIFY the guest requirement (menu-bar surface now exists; `share --json` contract)
- [x] `make check` + `cd macapp && swift build -c release` + `scripts/check-design.sh` green
- [ ] Archive `web-shared-input-menubar` once merged
