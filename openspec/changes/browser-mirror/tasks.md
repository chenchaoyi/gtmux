## 1. Web frontend (embedded SPA)

- [x] 1.1 Emit a browser-target terminal asset: extend `mobileapp/scripts/gen-xterm-asset.mjs` (or a sibling generator) to produce a browser bundle — xterm.js + fit + unicode11 inlined, plus the shared rendering core (the `⏺`→`●` glyph fix, wrap/no-wrap extent logic, cursor decoration) — usable directly (page calls `term.write()`), not via RN injection.
- [x] 1.2 Build `index.html` + `app.js` (framework-free): pairing layer that reads `#c=<code>` from the fragment, redeems it via `POST /api/enroll`, stores the device token (cookie/localStorage), and strips the fragment; show a "get a fresh link" state on expired/invalid codes.
- [x] 1.3 Agent radar: poll `GET /api/agents`, render the list with the shared status language (color+shape+glyph; section order waiting→working→idle→running); clicking a session opens its pane.
- [x] 1.4 Pane mirror: poll `GET /api/pane?id=%N`, `term.write()` the snapshot; xterm `fit()` on window resize; NO input/send/focus controls (view-only).
- [x] 1.5 Place the built static assets under `internal/server/web/` for embedding.

## 2. Serve: embed, routes, banner

- [x] 2.1 `//go:embed` the `web/` assets in `internal/server`; add `GET /` and `/assets/*` routes (static page is unauthenticated; the page then authenticates `/api/*` with the device token). Confirm `/api/*` contract is unchanged. Because both `serve` and `tunnel` run this server, the `/` route lights up on both automatically.
- [x] 2.2 Advertise the browser in BOTH banners: `printServeBanner` (`internal/app/serve.go`) prints the LAN browser URL(s) (via `reachableHosts`); `gtmux tunnel`'s pairing output prints the public HTTPS browser URL (`https://gtmux-<id>.ccy.dev/`). Both print a one-time pairing link (`/#c=<minted-code>`). All strings en+zh via `internal/i18n`.
- [x] 2.3 Gate: `make check` green (gofmt + vet + staticcheck + `go test -race`) and `CGO_ENABLED=0 go build ./cmd/gtmux` passes (cgo-free preserved).

## 3. Phone → computer handoff (mobile app)

- [ ] 3.1 Add an "Open on computer / 在电脑上打开" action in the mobile app: call `POST /api/enroll/mint`, build the pairing URL against the active server's public/LAN base, and present it via the iOS share sheet (AirDrop/copy/Messages). en/zh.
- [ ] 3.2 Gate: mobile `tsc --noEmit` + `eslint .` clean.

## 4. Verify (manual — needs a real browser; handoff needs a phone + Mac)

- [x] 4.1 LAN: open `http://<ip>:<port>/#c=<code>` in a desktop browser → pairs → radar shows agents → open a session → live mirror updates; resize the window → terminal refits, source pane width unchanged.
- [ ] 4.2 Tunnel: same flow via `https://gtmux-<id>.ccy.dev/` with `gtmux tunnel` running.
- [ ] 4.3 Handoff: on the paired phone tap "Open on computer" → share the link → open it on a Mac browser → it continues on the same agents.
- [x] 4.4 View-only confirmed: the web UI exposes no way to type into, send keys to, or focus a pane.

## 5. Ship

- [ ] 5.1 Branch → PR → CI green → squash-merge (never commit to main).
- [ ] 5.2 Sync the spec: `openspec` archive/sync so `openspec/specs/browser-mirror/` reflects the shipped behavior; `npx @fission-ai/openspec validate --specs --strict` passes.
