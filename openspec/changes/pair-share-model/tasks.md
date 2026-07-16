# Tasks: pair-share-model

## 1. S1 — per-link share scope (server + CLI)

- [x] 1.1 `EnrolledDevice` gains `ViewPanes/InputPanes []string` + `ExpiresAt
      int64` (guest-only; owner devices nil/0). Input⊆view normalized on write.
      Expiry enforced in `TokenScope` (past expiry = unknown token → 401).
- [x] 1.2 Auth carries the caller: guest requests stash their `EnrolledDevice` in
      the request context; guest checks become per-link (`CanViewFor`/
      `AllowedFor`, consent stays global). agents filter / pane / send / SSE /
      web mirror / share capability all resolve from the caller's link.
- [x] 1.3 Migration + template/broadcast: serve start copies legacy global lists
      into scope-less guest entries once; `share new` without flags copies the
      template; legacy global mutations update the template AND fan out to all
      links (CLI prints a fans-out notice).
- [x] 1.4 CLI: `share new --view/--type/--expires`, `share set <id> …`,
      per-link summaries in `share status` (+ additive `--json` fields
      `view_panes/panes/expires_at`). Server: `/api/share/new` accepts optional
      scope (additive).
- [x] 1.5 Tests: two-links-two-scopes (read + send), migration one-time copy,
      broadcast fan-out, template default, expiry 401, input⊆view per link,
      owner unaffected. Contract: api/contract.md share sections.

## 2. S2 — pair command + attach owner flow

- [x] 2.1 `gtmux pair`: mint one code → print QR + browser URL + attach
      one-liner; `pair list` (owner devices only) / `pair revoke <id>`;
      `gtmux devices` kept as alias. CLAUDE.md command list + help.go + docs/cli.md.
- [x] 2.2 attach: `#c=<code>` target → POST /api/enroll (name=hostname) → persist
      `~/.config/gtmux/remotes.json` (0600) → owner attach; bare `attach <host>`
      consults remotes.json. Tests: target parsing, redemption flow (fake server),
      persistence + reuse, revoked token → 401 path.

## 3. S3 — menu-bar two-track Preferences

- [ ] 3.1 Preferences reorg: 远程访问 / 你的设备 Pair (device rows + one pairing
      sheet, three media) / 分享 Share (consent + per-link rows with inline
      See/Type editor + New-share sheet with name+scope).
- [ ] 3.2 ShareStore/PairStore: per-link ops via `share set/new`; parse per-link
      scope from `share status --json`. Design-conformance test additions.

## 4. S4 — vocabulary + app/web polish

- [ ] 4.1 Mobile: server list split 我的 Mac / 访客连接; guest scope line; copy
      pass (配对 vs 分享). Jest for the list split.
- [ ] 4.2 Web guest page: "your access" strip from GET /api/share.
- [ ] 4.3 Cross-surface copy audit (menubar popover Pair button label ok; docs).

## 5. Consistency + verification

- [ ] 5.1 Fold spec deltas (remote-access, remote-terminal-client, menu-bar-app,
      mobile-app); `openspec validate --specs --strict` green; archive change.
- [ ] 5.2 make check + CGO_ENABLED=0 + mobile `npm run check` green.
- [ ] 5.3 Dogfood: pair a second computer's terminal via the one-liner; mint two
      links with different scopes and verify isolation from a browser.
