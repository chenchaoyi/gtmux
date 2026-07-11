# gtmux mobile — test plan (simulator)

Comprehensive test design for the gtmux iOS app, driven from the **iOS simulator**
with the real app and a real `gtmux serve` over the same Mac. The emphasis is the
in-app **terminal** (the native `<Text>` renderer — `NativeTerm`) — render, wrap,
horizontal/vertical scroll, live updates, font, fullscreen — plus the surrounding
flows (radar, pairing, settings, detail actions).

This doc is the source of truth for **manual review** and **regression**: each case
has an id, steps, and expected result; automatable cases name the e2e test that
covers them (`e2e/__tests__/*.test.ts`). Status column is filled during an
execution pass (`✅ pass` / `❌ defect #NNN` / `➖ n/a`).

## How to run

```bash
# 1. a booted sim + a live serve with agents on :8765 (the dev's own tmux works)
xcrun simctl boot "iPhone 17 Pro"
# 2. build + install the app fresh on the sim
cd mobileapp && npm run e2e:build
# 3. run the suite against the live serve
#    (terminal cases just switch Detail to Terminal mode — the native renderer is
#     always on now; there is no xterm toggle to set up)
GTMUX_E2E_URL=http://127.0.0.1:8765 \
GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
npm run test:e2e
```

Notes / harness facts:
- The terminal is native RN `<Text>`, so Appium's native context can read the
  rendered lines directly. Scroll/offset behaviour is still best verified by
  **screenshot diff** + **gesture drives** (there's no DOM `scrollLeft`); the
  native `ScrollView` clamps content, and `NativeTerm` measures its own columns.
- Terminal content depends on the live panes; for **wide-line** cases use a pane
  whose output has lines ≥ ~120 cols (code/listings), or inject one into a throwaway
  agent pane (never into the dev's working panes).

---

## TERM — terminal (native `<Text>`) rendering

| id | title | steps | expected | method | status |
|----|-------|-------|----------|--------|--------|
| TERM-01 | renders with ANSI color | open a colored pane, Terminal mode | text shows true terminal colors (fg/bg/bold), matching the pane | visual | |
| TERM-02 | CJK / wide-glyph width | view a pane with 中文/emoji | wide glyphs occupy 2 cells; columns stay aligned | visual | |
| TERM-03 | full-screen TUI content | view a pane running a TUI snapshot | grid renders without overlap/corruption | visual | |
| TERM-04 | empty / short pane | view a near-empty pane | renders cleanly, no error, no stray scroll | visual | |
| TERM-05 | glyph normalization | view a pane emitting ⏺/⏸/⚠ | record dot → ●, bare text-default symbols render as text (not color emoji), matching the terminal | visual | |

## TERM-WRAP — wrap toggle

| id | title | steps | expected | method | status |
|----|-------|-------|----------|--------|--------|
| WRAP-01 | wrap ON (default) wraps long lines | open detail (Wrap shown) | long lines wrap to the next row; no horizontal scroll | visual | |
| WRAP-02 | toggle → Scroll (no-wrap) | tap Wrap → label becomes Scroll | long lines extend past the right edge (cut off), not wrapped | visual | |
| WRAP-03 | toggle back re-wraps | tap Scroll → Wrap | lines wrap again; horizontal disabled | visual | |
| WRAP-04 | wrap state re-renders content | toggle while content present | content stays correct after each toggle (no blank/garbled) | visual | |
| WRAP-05 | wrap renders on FIRST open (regression #141) | open detail fresh (no toggle) | terminal shows text immediately — NOT a black screen (the cursor decoration must not blank the WebGL layer in wrap mode) | visual | |

## TERM-HSCROLL — horizontal scroll (no-wrap)

| id | title | steps | expected | method | status |
|----|-------|-------|----------|--------|--------|
| HSCROLL-01 | horizontal swipe scrolls | no-wrap, swipe right→left on the pane | content scrolls left, revealing the right of long lines | appium (screenshot A/B) | |
| HSCROLL-02 | bounded at the longest line | swipe hard to the end | stops at the last column; no scroll into empty space | appium + webkit-measure | |
| HSCROLL-03 | triggers when scrolled up | scroll up ≥3 screens, then horizontal swipe | horizontal scroll still triggers (axis lock) | appium (screenshot A/B) | |
| HSCROLL-04 | no vertical bleed | horizontal swipe (with slight vertical) | vertical position holds; only horizontal moves | appium (screenshot A/B) | |
| HSCROLL-05 | narrow content doesn't h-scroll | no-wrap, content fits width | no horizontal scrolling (nothing off-screen) | visual | |

## TERM-VSCROLL — vertical scroll

| id | title | steps | expected | method | status |
|----|-------|-------|----------|--------|--------|
| VSCROLL-01 | smooth scrollback | vertical swipe up/down | scrolls smoothly through history (WebGL, no jank) | manual/visual | |
| VSCROLL-02 | up shows earlier, bottom shows latest | swipe down (history) then up | earlier content then back to live tail | appium (screenshot) | |
| VSCROLL-03 | momentum / inertia | flick vertically | inertial scrolling | manual | |
| VSCROLL-04 | vertical swipe ≠ horizontal | pure vertical swipe in no-wrap | only vertical moves (axis lock) | appium (screenshot) | |

## TERM-LIVE — live updates (working pane)

| id | title | steps | expected | method | status |
|----|-------|-------|----------|--------|--------|
| LIVE-01 | append without flash | watch a streaming pane | new lines append; no full-screen flash each poll | manual/visual | |
| LIVE-02 | follows tail at bottom | sit at bottom of a working pane | new output keeps the latest visible | visual | |
| LIVE-03 | scroll position held on update | scroll up, wait for an update | not yanked back to bottom | manual | |
| LIVE-04 | writes paused during a gesture | scroll while output streams | the in-progress scroll isn't interrupted by a redraw | manual | |

## TERM-FONT / FULLSCREEN

| id | title | steps | expected | method | status |
|----|-------|-------|----------|--------|--------|
| FONT-01 | A+ enlarges | tap A+ | font grows; cols re-fit | visual | |
| FONT-02 | A− shrinks | tap A− | font shrinks; more cols fit | visual | |
| FS-01 | fullscreen expands | tap fullscreen | pane fills screen; controls hidden | visual | |
| FS-02 | exit fullscreen | tap exit | returns to detail with controls | visual | |

## DETAIL — pane actions

| id | title | steps | expected | method | status |
|----|-------|-------|----------|--------|--------|
| DET-01 | back to radar | tap detail-back | returns to radar | appium (smoke) | |
| DET-02 | composer send | type + send | text reaches the pane (`/api/send`) | appium | |
| DET-03 | diff modal | tap Diff | shows the pane cwd's git diff (or empty state) | visual | |
| DET-04 | floating keys | open keys, send one | named key reaches the pane | manual | |

## RADAR / PAIR / SETTINGS

| id | title | steps | expected | method | status |
|----|-------|-------|----------|--------|--------|
| RADAR-01 | lists agents w/ status | open radar | agents grouped needs-you→working→idle; counts correct | appium (radar.test) | |
| RADAR-02 | open first agent | tap first row | Detail opens | appium (radar.test) | |
| RADAR-03 | server chip → servers page | tap chip | multi-server connection page | appium (screenshots.test) | |
| PAIR-01 | enroll-code (v2) pairing | scan a v2 QR | redeems code → per-device token, connects | manual (device) | |
| PAIR-02 | manual host+token | enter host+token | connects | appium | |
| PAIR-03 | remove server confirms | Settings → Remove | confirmation alert before removal | appium | |
| SET-01 | language en/zh/system | switch language | UI strings switch immediately | appium | |
| SET-02 | default detail mode | Settings → Terminal → Default mode | Detail opens in the chosen mode (Terminal/Chat) next time | appium | |
| SET-03 | push toggle | toggle push | persists | appium | |

---

## Execution log

Each pass appends a dated section with the result of every id and links to the
defects it filed.

### 2026-06-25 — pass 1 (iPhone 17 Pro sim, app paired to the dev's live serve, xterm on)

Driven via Appium (`qa-drive*.mjs`, scratch) + screenshot review. Pane under test:
`gtmux.app dev` (this session — long code/path lines, good for no-wrap/h-scroll).

| id | result | note |
|----|--------|------|
| TERM-01 color | ✅ | cyan/green/pink render true to the terminal |
| TERM-02 CJK | ✅ | 中文 lines render, columns aligned (unicode11) |
| WRAP-01 wrap on | ✅ | long lines wrap |
| WRAP-02 no-wrap | ✅ | control → "Scroll"; lines extend past the right edge |
| WRAP-03 wrap again | ✅ | re-wraps cleanly |
| HSCROLL-01 h-swipe | ✅ | reveals the right of long lines |
| **HSCROLL-02 bound** | **❌→✅** | **DEFECT: hard swipe right went fully BLACK — extent was the longest line across ALL scrollback, so short visible lines left a void. FIXED: extent now tracks the widest VISIBLE line (`visibleMaxCols`), recomputed on vertical scroll. Re-tested: hard swipe keeps content, stops at the visible line end.** |
| HSCROLL-03 trigger when scrolled up | ✅ | axis lock — works off the bottom |
| HSCROLL-04 no vertical bleed | ✅ | same vertical region held during h-swipe |
| VSCROLL-02 up/bottom | ✅ | scroll up shows earlier history |
| FONT-01 A+ | ✅ | enlarges, re-fits |
| FONT-02 A− | ✅ | shrinks, more content fits |
| DET-03 diff | ✅ | git-diff modal renders colored |
| FS-01/02 fullscreen | ➖ | not executed (test coordinate hit the composer — re-do with a stable testID) |
| LIVE-01..04 | ➖ | not executed this pass (needs a streaming pane + flash detection) |

**Defects fixed this pass:** the horizontal over-scroll-into-emptiness (HSCROLL-02).
**Follow-ups:** add testIds to the detail controls (Wrap/A±/fullscreen) so the e2e
suite can target them deterministically; add a LIVE streaming-pane case; consider
a faint horizontal scroll indicator for very wide content.

### 2026-06-26 — pass 2 (iPhone 17 Pro sim, xterm on) — WRAP regression sweep

Triggered by a user report: "wrap doesn't work." Rebuilt the sim from `main`
(the prior pass had run a *stale* asset predating #140) and re-ran the WRAP set.

| id | result | note |
|----|--------|------|
| **WRAP-05 first-open render** | **❌→✅** | **DEFECT (#141): wrap mode — the default — showed a fully BLACK terminal on first open; only the cursor box drew. Root cause: #140's cursor *decoration* leaves xterm's WebGL base layer blank until the next repaint; no-wrap repainted via its per-poll `relayoutCols` refit, but wrap never refits. FIXED: `term.refresh()` (next animation frame) after the cursor decoration changes. Re-tested: wrap renders text at open.** |
| WRAP-01/03/04 | ✅ | re-verified after the fix — wrap renders, toggles cleanly, no blank |

**Defects fixed this pass:** wrap-mode black-screen-on-open (#141).
**Process note:** always rebuild the sim from `main` before a regression pass —
a stale installed `.app` masked this (pass 1 ran #139's asset, not #140's).
