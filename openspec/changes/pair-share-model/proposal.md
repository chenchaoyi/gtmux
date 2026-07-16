# Proposal: pair-share-model

## Why

gtmux's remote credentials already split mechanically into two kinds — device
tokens (scope `""`, the owner's own surfaces) and guest tokens (scope `"guest"`,
share links) — but the product concept was never established, and the halves have
uneven capabilities (commander-directed redesign, 2026-07-16):

1. **Share's core gap: allowlists are GLOBAL.** `ShareState.Panes/ViewPanes` is
   one set shared by every guest link — "let Alice view session A while Bob
   operates session B" is impossible. The menu-bar flow (mint a link, then tick
   panes globally) reflects and cements this.
2. **Pair has no unified story.** Pairing a phone (QR), a browser (`#c=` code),
   and another computer's terminal are three unrelated flows — and the terminal
   one doesn't exist at all (`gtmux attach` as owner requires hand-carrying the
   host + master token, while the GUEST path via share link is smooth: the
   experience is inverted).
3. **Vocabulary is scattered**: "Remote access", "Shared input", "Pair" (popover
   button), `gtmux devices` vs `gtmux share` — no surface names the two-track
   model.

The agreed model: **Pair = yourself** (full control — browser, terminal, app; a
long-lived second screen of the owner) vs **Share = a collaborator** (least
privilege — chosen sessions, view vs operate per session, revocable, optionally
expiring). Decisions confirmed: migrate the legacy global allowlist by copying it
into every existing link once; expiry defaults to never (optional `--expires`);
`gtmux pair` becomes the primary command with `gtmux devices` kept as an alias.

## What Changes

Four slices under one model:

- **S1 — per-link share scope (server + CLI).** Each guest token carries its OWN
  view/input allowlists and optional expiry. The legacy global lists are migrated
  into existing links once, then kept as (a) the default template for new links
  and (b) a broadcast edit surface (`gtmux share add %N` fans out to all links),
  so the pre-S3 menu-bar UI keeps its exact semantics. New: `gtmux share new
  --view/--type/--expires`, `gtmux share set <id> …`, per-link scope in
  `share status`. `GET /api/share` (the caller's own capability) keeps its shape —
  guests now resolve from their token's scope; apps/web need no change. Expired
  links authenticate as invalid (401).
- **S2 — pair as a first-class flow.** New `gtmux pair` command (mint a pairing
  code rendered three ways: QR for the phone, URL+code for a browser, and a
  one-line `gtmux attach` command for a terminal); `pair list`/`pair revoke`
  subsume the device roster (`gtmux devices` stays as an alias). `gtmux attach`
  learns the pair flow: an attach target carrying an enroll code redeems it for
  an OWNER device token and persists it locally, so later `gtmux attach <host>`
  just works.
- **S3 — menu-bar Preferences reorganized** into the two-track model: 远程访问
  (the door, unchanged) · 你的设备 Pair (paired-device list + one pairing sheet
  with the three media) · 分享 Share (consent master switch + per-link list with
  an inline scope editor + a new-share sheet that names the link AND selects its
  sessions in one step).
- **S4 — vocabulary + surface polish**: mobile app splits "my Macs (pair)" from
  "guest connections (share)" and shows a guest's granted scope; the web guest
  page gains a "your access" strip; copy across surfaces standardizes on
  Pair/配对 vs Share/分享.

## Capabilities

### Modified Capabilities
- `remote-access`: guest allowlists become per-link (with the one-time migration
  + template/broadcast compatibility semantics), optional link expiry, the
  `gtmux pair` command + terminal pair flow via attach, and the pair/share
  vocabulary for credential scopes.
- `menu-bar-app`: Preferences two-track reorganization (pair list + pairing
  sheet; share list + per-link scope editor + creation sheet).
- `mobile-app`: pair/share split in the server list and guest-scope display.
- `remote-terminal-client`: attach's owner-pairing flow (enroll-code redemption
  + local token persistence).

## Impact

- Server: `internal/server/enroll.go` (per-device scope fields + expiry),
  `share.go` (per-token checks + template/broadcast), `server.go` (auth carries
  caller identity; scoped checks), share/attach handlers.
- CLI: `internal/app/share.go` (new flags/subcommands), new `internal/app/pair.go`,
  `devices.go` (alias), attach target parsing + local remote-token store.
- macapp: `Preferences.swift`, `ShareStore.swift` (+ pair store).
- mobileapp: server list + guest scope display (S4).
- Docs: `docs/cli.md` (share/pair/attach), `api/contract.md` (share endpoints’
  additive fields), CLAUDE.md command list (`pair`).
- Specs: remote-access, menu-bar-app, mobile-app, remote-terminal-client.
