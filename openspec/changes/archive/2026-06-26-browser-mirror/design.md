## Context

`gtmux serve` runs an HTTP server on the Mac with the agents, guarding `/api/*`
with a bearer token (master token + device tokens minted via `/api/enroll`). The
mobile app is the only consumer today. The tunnel (`gtmux tunnel`) already gives a
public `https://gtmux-<id>.ccy.dev/` hostname. There is **no web page** at `/`.

We want a second Mac (or any browser) to watch the agents live with zero install.
The renderer to reuse is the xterm.js bootstrap in
`mobileapp/scripts/gen-xterm-asset.mjs`, but it's written for the RN-webview
injection model (`gtmuxWrite`/`gtmuxConfig` pushed via `injectJavaScript`); a
browser page must instead fetch the API itself.

Constraints: the CLI must stay **cgo-free**; the embedded asset must work
**offline** (no CDN); CLI strings are **en+zh** via `internal/i18n`; the status
language (color+shape+glyph, section order waiting→working→idle→running) must match
the other surfaces.

## Goals / Non-Goals

**Goals:**
- View-only browser UI served by `gtmux serve` at `/`, zero install on the viewer.
- Agent radar + live pane mirror (xterm.js) that fits the browser window.
- One-time pairing link; master token never in a URL.
- Works on LAN and via the existing tunnel; offline-safe embedded asset; cgo-free.

**Non-Goals (v1):** input/`send-keys`, focus, a read-only token scope at the auth
layer, SSE/WebSocket, remote pane resize, multi-pane/split. (All v2.)

## Decisions

- **Serve the web UI from the shared HTTP server (`internal/server`); no new
  command.** Both `gtmux serve` and `gtmux tunnel` run this same server (`tunnel`
  starts it in-process via `startLocalRadar`, bound to loopback, then exposes it
  through cloudflared). So adding a `/` route to the server makes the web UI
  reachable on BOTH paths for free: `serve` → LAN (`http://<lan-ip>:<port>/`),
  `tunnel` → any network (`https://gtmux-<id>.ccy.dev/`). _Alt:_ a dedicated
  `gtmux mirror` command — rejected: it'd duplicate serve/tunnel lifecycle for no
  gain. Consequence: BOTH banners (serve's `printServeBanner` and tunnel's pairing
  output) must advertise the browser URL + a one-time pairing link.

- **Embed a framework-free vanilla-JS SPA via `//go:embed`.** One `index.html` +
  one JS file + the inlined xterm.js/CSS, served from the binary. _Alt:_ a React/
  Vite build — rejected: adds a build step + node toolchain to a Go release, risks
  the offline/cgo-free invariants. Vanilla embeds cleanly and the UI is small
  (a list + a terminal).

- **Reuse the xterm rendering core, swap the data layer.** Factor the terminal
  setup (Terminal opts, fit/unicode11 addons, the `⏺`→`●` glyph fix, the cursor
  decoration, the wrap/no-wrap logic) so the browser page can `term.write()` from
  its own `fetch('/api/pane')` poll instead of RN injection. _Alt:_ rebuild the
  renderer from scratch — rejected: throws away the hard-won mobile tuning.

- **Pairing = one-time enroll-code link → cookie.** `serve` prints
  `http://host:port/#c=<code>` (code = the existing 5-min single-use enroll code).
  The page reads the `#c=` fragment, POSTs it to `/api/enroll`, gets a device token,
  stores it in a `Secure`/`SameSite` cookie (or localStorage), and strips the
  fragment from the URL. _Alt:_ paste-the-token — rejected: clunky cross-machine
  copy + tempts putting the master token in a URL. The page is served same-origin
  as the API, so **no CORS** and the cookie/Authorization just works.

- **Phone → computer handoff via `/api/enroll/mint`.** The same code mechanism, but
  the code is minted by an already-paired phone (which is authenticated) rather than
  printed by the banner. The phone builds `<public-url>/#c=<code>` and offers it via
  the iOS share sheet (AirDrop/copy/Messages); the computer opens it and pairs.
  Reuses `EnrollManager.Mint()` (5-min, single-use) exactly as the QR pairing does —
  **no new endpoint**; the only new code is a phone-app action + the browser page's
  `#c=` handling (shared with the banner path). This is expected to be the primary
  flow for a single user (watching on the phone, moving to the big screen).

- **Live updates by polling `/api/pane` (~1–1.5s), like the phone.** Reuses the
  existing endpoint; the browser tab is foreground-only so the cost is bounded.
  _Alt:_ SSE/WebSocket now — deferred to v2 (lower latency, but more server work).

- **Window resize = xterm `fit()` of the VIEW; do not resize the remote pane.**
  The mirror shows Mac A's pane at its real width; a wider browser just shows it
  with less wrapping / more margin. _Alt:_ drive `tmux resize-window` to reflow the
  agent to the browser — rejected for v1 (mutates the source Mac; risky for agents).

- **i18n:** the embedded page follows the browser language (en/zh) for its own
  chrome; CLI banner strings stay en+zh via `internal/i18n`.

## Risks / Trade-offs

- **The bearer token is RCE (`/api/send`) even though the UI is view-only.** →
  Mitigation: v1 ships **no** control affordances in the browser, keeps the
  existing "behind VPN/tunnel" guidance, and the pairing link uses a short-lived
  code (revocable device token), not the master token. A real read-only token
  scope is the first v2 task. Document this clearly.

- **LAN transport is HTTP (token/cookie in cleartext on the LAN).** → Mitigation:
  recommend the tunnel (HTTPS) for anything beyond a trusted home LAN; note in the
  banner.

- **Reusing the mobile bootstrap risks coupling two consumers to one asset.** →
  Mitigation: factor a small shared core; the browser page owns its own data/poll
  layer so the RN side is untouched. Keep the generator producing both targets.

- **Short-lived pairing code can expire before the viewer opens the link.** →
  Mitigation: regenerate on demand (re-running `serve`/a key reprints it); the
  master token still works as a fallback for the technical user.

- **Embedded xterm.js inflates the binary (~300KB).** → Acceptable; it's inlined
  once and gzipped on the wire.
