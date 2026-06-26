## Why

`gtmux serve` already exposes the agent radar + live panes over HTTP, but the only
consumer is the mobile app. A person on a second Mac (or any computer) has no way
to watch another Mac's tmux agent sessions without installing something. A browser
is the universal, zero-install client — and a desktop browser is where a terminal
renders best (resizable window, full keyboard later). This adds a read-only web UI
served by `gtmux serve` so anyone with a link can open a browser and watch the
agents live.

## What Changes

- The shared HTTP server (`internal/server`) ALSO serves a small **view-only web
  UI at `/`** (alongside the existing `/api/*`). No new command — and because both
  `gtmux serve` and `gtmux tunnel` run this same server (`tunnel` starts it
  in-process via `startLocalRadar`), the web UI rides BOTH paths automatically:
  `serve` exposes it on the **LAN**, `tunnel` exposes it over **any network** via
  the Cloudflare hostname.
- The web UI shows the **agent radar** (same status language) and, on selecting a
  session, a **live pane mirror** rendered with xterm.js that **fits the browser
  window** (resize freely; wider window = cleaner, less wrapping than the phone).
- **One-time pairing link**: the `serve` banner prints a browser URL plus a
  short-lived pairing link (`http://host:port/#c=<code>`). Opening it in a browser
  exchanges the code (via the existing `/api/enroll`) for a device token stored in
  a cookie — the master token never travels in a URL.
- **Phone → computer handoff (the nicer path)**: an already-paired phone can mint a
  fresh code (via the existing authenticated `/api/enroll/mint`) and **share a
  pairing link** (iOS share sheet → AirDrop/copy) so you continue on your computer's
  bigger screen. No new server endpoint — adds one "open on computer" action in the
  mobile app.
- Works on the **LAN** (`http://<ip>:<port>/`) and **remotely** via `gtmux tunnel`
  (`https://gtmux-<id>.ccy.dev/`).
- The frontend is **framework-free vanilla JS with xterm.js inlined**, embedded in
  the Go binary via `//go:embed` — no build step, the CLI stays cgo-free.
- Live updates by **polling `/api/pane`** (same model as the phone).

### Non-goals (v1)

- **No input / control**: no typing, `send-keys`, or focus from the browser — it is
  strictly view-only. (`/api/send` is RCE; control is v2 behind a separate token.)
- **No read-only token scope** at the auth layer yet — v1 is safe-by-UI (no control
  affordances) + the existing "keep behind VPN/tunnel" guidance. The bearer token
  technically still permits `/api/send`; tightening that is v2.
- No SSE/WebSocket live channel, no remote pane resize, no multi-pane/split view.

## Capabilities

### New Capabilities
- `browser-mirror`: a view-only web UI served by `gtmux serve` that lets a browser
  on another machine pair via a one-time link and watch the agent radar + a live,
  window-resizable pane mirror (xterm.js), over LAN or tunnel.

### Modified Capabilities
<!-- none — the existing remote-access API (/api/agents, /api/pane, /api/enroll) is
     reused unchanged; the new `/` route + banner line belong to browser-mirror. -->

## Impact

- **Code**: `internal/server` (new `/` + static asset routes via `//go:embed`;
  reuses existing `/api/*` incl. `/api/enroll` + `/api/enroll/mint`),
  `internal/app/serve.go` (banner prints browser URL + pairing link), a new
  embedded web frontend (vanilla JS + inlined xterm.js, reusing the rendering core
  from `mobileapp/scripts/gen-xterm-asset.mjs` where practical), and a small
  **mobile-app** addition (an "open on computer" action: mint a code + share the
  pairing link).
- **APIs**: no change to the `/api/*` contract; the browser is a new consumer of
  `/api/agents`, `/api/pane`, and `/api/enroll`.
- **Dependencies**: none new on the Go side (cgo-free preserved); xterm.js is
  vendored/inlined into the embedded asset.
- **Security**: the pairing link carries a short-lived enroll code, not the master
  token; transport is HTTPS via the tunnel (HTTP on trusted LAN). View-only by UI.
