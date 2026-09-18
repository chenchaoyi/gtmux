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

## After a UI change

The artwork reuses whatever is in `mobileapp/.e2e-artifacts/appstore/`, so rendering it
alone repaints the frames around **last time's** device screens. When the app itself
changed, re-shoot those first — `docs/appstore-shots.md` is the whole procedure, and the
iPad half is in `docs/appstore/submit.md`:

```sh
cd mobileapp
GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID=<booted sim> \
  npm run test:e2e -- appstore-shots.test.ts          # the 6 phone screens
GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID=<booted iPad sim> \
  GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad.test.ts
cd .. && bash docs/assets/screenshots/regenerate.sh   # docs shots + artwork
```

Which images a change makes stale:

| What changed | Re-shoot |
|---|---|
| any app screen in the artwork (radar, a pane reply, HQ, usage, iPad split) | the store e2e, then the artwork |
| the Detail or connection page | `regenerate.sh` (its own simulator pass) |
| the web page (`internal/server/web`) | the artwork alone — it captures the page live |
| `gtmux agents` output, or the menu-bar popover's layout or palette | the artwork alone, and check `menubar-panel.html` still matches `Theme.swift` |
| the demo fleet in `mock-serve.js` or `demoData.ts` | everything |

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

## Prerequisites

- **Google Chrome** — headless, it renders every HTML template. The only thing the
  artwork needs.

Mobile capture also needs:

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
