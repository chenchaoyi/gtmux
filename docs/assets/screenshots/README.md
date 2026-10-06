# Regenerating the docs screenshots

The script below regenerates the six documentation images listed here, using generic
fixtures and existing App Store demo captures. Run it from the repository root after
preparing the inputs and prerequisites below:

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
  ([phone procedure](../../appstore-shots.md), [iPad procedure](../../appstore/submit.md)).
- **Browser** — the actual page from `internal/server/web`, loaded by headless Chrome
  against `mock-serve.js`, which serves both the page and the fleet it shows.
- **Terminal** — drawn, but its text is what `gtmux agents` prints for that same fleet:
  the block the README shows, which `TestREADMEAgentsSampleIsReal` compares against the
  real renderer.
- **Menu bar** — drawn, from the app's own measurements. `menubar-panel.html` lists
  values taken from `Theme.swift` and `MenuView.swift` and explains why the original capture environment
  could not record the menu bar. This drawing needs
  a manual comparison when the app changes; it is not evidence of a current native UI test.

Render only the artwork, no simulator needed, with
`GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh`. The design canvas the
composition came from is under `docs/design/mockup/readme-artwork/`.

## After a UI change

The artwork reuses whatever is in `mobileapp/.e2e-artifacts/appstore/`, so rendering it
alone repaints the frames around **last time's** device screens. When the app itself
changed, re-shoot those first — [the phone procedure](../../appstore-shots.md) covers
language, the unpaired starting state and checking that all six captures are fresh;
[the submission guide](../../appstore/submit.md) covers iPad. After those preparations:

```sh
(
  set -eu
  cd mobileapp
  : "${AUDIT_SIM_UDID:?the prepared phone simulator}" "${AUDIT_IPAD_UDID:?the prepared iPad simulator}"
  GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID="$AUDIT_SIM_UDID" \
    npm run test:e2e -- appstore-shots.test.ts
  GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID="${AUDIT_IPAD_UDID:?the prepared iPad simulator}" \
    GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad.test.ts
  node scripts/render-lockscreen.mjs --lang en --out .e2e-artifacts/appstore/en
)
```

After checking that every raw image is fresh and depicts the intended screen:

```sh
GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh
```

Which images a change makes stale:

| What changed | Re-shoot |
|---|---|
| any app screen in the artwork (radar, a pane reply, HQ, usage, iPad split) | the store e2e, then the artwork |
| the Detail or connection page | `regenerate.sh` (its own simulator pass) |
| the web page (`internal/server/web`) | the artwork alone — it captures the page live |
| the lock-screen widget | re-render `02-lockscreen.png`, compare the drawing with the widget, then the artwork |
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

- **macOS tools** — `sips`, `curl`, `sed`, and Bash; **Node** for the fixture server.
- **Google Chrome** at `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`;
  it renders the HTML templates headlessly.
- **Port 8799 available** for the script's fixture server.
- **Existing raw images** under `mobileapp/.e2e-artifacts/appstore/`:
  `en/01-radar.png`, `en/02-lockscreen.png`, `en/02-terminal-approval.png`,
  `en/03-hq.png`, `en/05-usage.png`, and `ipad-en/01-split.png`.
  `GTMUX_ONLY=readme` requires these too; it does not capture or refresh them.

Mobile capture also needs:

- A **booted, dedicated iOS simulator** with the app installed. Set `GTMUX_E2E_UDID`
  explicitly and `GTMUX_E2E_OS` to its runtime (this script otherwise defaults to 26.4).
  First time / after app changes, build it as described in the
  [e2e guide](../../../mobileapp/e2e/README.md), or pass `GTMUX_SHOTS_BUILD=1`.
- Appium's xcuitest driver (one-time): `cd mobileapp && npx appium driver install xcuitest`.
- Use `GTMUX_E2E_SOFT_KEYBOARD=1` if the capture needs the software keyboard.
- A first WebDriverAgent build can take a few minutes; the harness waits.

Only need the README artwork (no simulator)?

```sh
GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh
```

The full run replaces `mobileapp/.e2e-artifacts/shots/` and updates the two phone-document PNGs
as well as the four README JPEGs. After running, inspect all changed images and review
`git status --short docs/assets/` before committing them.
