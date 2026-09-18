# Regenerating the docs screenshots

One command re-renders every user-facing screenshot with generic data (no real
session names, file paths, server name or token cost):

```sh
bash docs/assets/screenshots/regenerate.sh
```

It produces, under `docs/assets/`:

| Image | Used in | How it's made |
|---|---|---|
| `readme-hero.jpg`, `readme-hero-dark.jpg` | README top image (light and dark) | `readme-hero.html`, one template for both themes |
| `readme-screens.jpg`, `readme-screens-dark.jpg` | README "What it looks like" | `readme-screens.html`, same |
| `screenshot-detail.png` | `docs/phone.md` | real simulator capture (Detail, Terminal) |
| `screenshot-servers.png` | `docs/phone.md` | real simulator capture (connection page) |

The top image carries all five surfaces, and each one is as real as it can be:

- **iPhone and iPad** — App Store demo-mode captures from
  `mobileapp/.e2e-artifacts/appstore/`, written by the `appstore-shots` e2e
  (`docs/appstore/submit.md`).
- **Browser** — the actual page from `internal/server/web`, loaded by headless Chrome
  against `mock-serve.js`, which serves both the page and the fleet it shows.
- **Terminal** — drawn, but its text is what `gtmux agents` prints for that same fleet:
  the block the README shows, which `TestREADMEAgentsSampleIsReal` compares against the
  real renderer.
- **Menu bar** — drawn, from the app's own measurements. `menubar-panel.html` says why
  and lists every value it took from `Theme.swift`. It is the one panel nobody can
  capture here: macOS screen recording is permission-blocked, and the real popover would
  show the owner's own sessions.

Render only the artwork, no simulator needed, with
`GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh`. The design canvas the
composition came from is under `docs/design/mockup/readme-artwork/`.

## How it works

- **`mock-serve.js`** — a throwaway HTTP server that answers the handful of
  `/api/*` endpoints the surfaces hit (`health`, `agents`, `pane`, `theme`, `options`,
  `share`, …) with the generic fixtures defined at the top of the file, and serves the
  real web page from `internal/server/web` with one added line that seeds the browser's
  token and board layout. Edit the fixtures to change what the screenshots show. It never
  touches your real tmux.
- The mobile shots come from the app's own **`GTMUX_SHOTS` e2e harness**
  (`mobileapp/e2e/__tests__/screenshots.test.ts`) pointed at the mock, with a
  generic server name (`GTMUX_SHOTS_NAME`, default `demo-mac`).

## Prerequisites (mobile capture only)

- A **booted iOS simulator** with the app installed. First time / after app
  changes: `cd mobileapp && npm run e2e:build` (or pass `GTMUX_SHOTS_BUILD=1`).
- Appium's xcuitest driver (one-time): `cd mobileapp && npx appium driver install xcuitest`.
- Hardware keyboard off on the sim (see `mobileapp/e2e/README.md`).
- A first WebDriverAgent build can take a few minutes; the harness waits.

Only need the README artwork (no simulator)?

```sh
GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh
```

After running, review with `git status docs/assets/` and commit the PNGs.
