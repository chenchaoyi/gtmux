# gtmux's five surfaces: every iteration walks through them

gtmux is one product with five surfaces, sharing one Go core (`internal/`, the single data
source) and one status language (waiting red / working cyan / idle green / running grey;
colour means status). Since 2026-09-12 the rule is: **every change to user-visible
behaviour says, in its proposal, what it does on each of the five**: done, not applicable
(and why), or left to a named later change. `scripts/check-design.sh` checks that every
in-flight proposal carries the section and that all five names appear; one missing is red.

| Surface | Where | What it is | Design authority |
|---|---|---|---|
| Terminal | `cmd/gtmux` + `internal/`; `gtmux attach` over `GET /api/attach` | The CLI itself, and remote attach: a remote tmux pane's PTY bridged to a local terminal (owner or guest) | `docs/cli.md`, `docs/design/remote-attach-research.md` |
| Menu bar | `macapp/` | Native Swift, a pure consumer of `agents --json`; the notification click target | `docs/design/DESIGN.md` |
| Phone | `mobileapp/`, the compact shell | iPhone: radar → detail → HQ as a stack; push; terminal input | `docs/design/MOBILE.md` |
| iPad | `mobileapp/`, the regular shell | The same app as a sidebar beside a main pane; hardware keyboard, pointer, multitasking windows | `docs/design/MOBILE.md` §5, change `ipad-universal-app` |
| Web | serve's share and pairing pages | A read-only mirror in a browser, with guest input behind the host's consent | `docs/design/WEB.md` |

## Why this is a written rule

The iPad split view was half built in 2026-07, deferred at the first store submission, and
for the two months after that every radar change landed on the phone only: the floating HQ
disc, the errored section, the guest banner, the long-press menu. Nobody decided to skip
the iPad; no step asked "and what does this do on the iPad", so the surface missing from
the list drifted without anyone noticing. The demo fell behind the real radar twice for the
same reason.

## Keeping drift out: structure first, then the checklist

Two things keep a surface from drifting: the proposal section above, which catches a surface
that was forgotten, and the four structural rules below, which catch a surface that was
copied instead of shared:

1. One implementation, several presentations. When the same thing appears on two
   surfaces it is one component plus a prop; a second copy is the defect. Phone and iPad:
   `RadarPanel` in two variants, `DetailView` / `HQView` / `PaneBrowserView` with a
   `layout` prop; `shellDrift.test.ts` reads the source to keep it so. Menu bar and phone
   share data contracts (`agents --json`, `/api/digest`) and no code, so `DESIGN.md` and
   `MOBILE.md` each carry a section aligning the status language.
2. Opening something does not know which surface it is on. "Open this pane / HQ /
   All panes" is workspace state (`WorkspaceContext`); the shell decides whether that
   becomes navigation or a main-pane switch. Push deep links, keyboard commands and
   buttons only set the state.
3. The demo is the real shells over fake data. Demo mode renders the same shells
   (compact / regular); only the client differs. It may not have a radar of its own.
4. The boundary between surfaces is written in the proposal. A change touching one
   surface only is normal (a menu-bar popover, say), but "not applicable" is said out
   loud, so a reviewer can disagree.

## What the section looks like

```
## Surfaces

- Terminal (incl. attach): not applicable — a layout change; the CLI has no counterpart.
- Menu bar: not applicable — the menu bar has no HQ console.
- Phone: unchanged; the compact shell is untouched.
- iPad: the subject of this change.
- Web: not applicable — the share page does not show HQ.
```

All five names must appear (终端 / 菜单栏 / 手机 / iPad / Web, or terminal / menubar /
phone / iPad / web); the gate checks presence only, the judgement stays the reviewer's.
