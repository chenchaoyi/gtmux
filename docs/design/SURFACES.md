# gtmux's five surfaces: every iteration walks through them

gtmux is one product with five surfaces, sharing one Go core (`internal/`, the single data
source) and one status language (waiting red / working cyan / idle green / running grey;
colour means status). Since 2026-09-12 the rule is: **every change to user-visible
behaviour says, in its proposal, what it does on each of the five**: done, not applicable
(and why), or left to a named later change. `scripts/check-design.sh` checks that every
in-flight proposal carries the section and that all five names appear; one missing is red.

| Surface | Where | What it is | Design authority |
|---|---|---|---|
| Terminal | `cmd/gtmux` + `internal/`; `gtmux attach` over `GET /api/attach` | The CLI itself, and remote attach: a remote tmux pane's PTY bridged to a local terminal (the owner or a paired device; a share link is refused) | `docs/cli.md`, `docs/design/remote-attach-research.md` |
| Menu bar | `macapp/` | Native Swift, a pure consumer of `agents --json`; the notification click target | `docs/design/DESIGN.md` |
| Phone | `mobileapp/`, the compact shell | iPhone: radar → detail → HQ as a stack; push; terminal input | `docs/design/MOBILE.md` |
| iPad | `mobileapp/`, the regular shell | The same app as a sidebar beside a main pane; hardware keyboard, pointer, multitasking windows | `docs/design/MOBILE.md` §5, change `ipad-universal-app` |
| Web | serve's share and pairing pages | A browser mirror with pane input only where the owner or a consented guest has permission | `docs/design/WEB.md` |

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

## Corresponding interaction audit (2026-10-10)

The audit covers the shared controls below; it does not claim device acceptance for
every screen. Native layouts and available permissions remain surface-specific.

| Interaction | Common meaning and order | Evidence / result |
|---|---|---|
| Agent detail modes | Chat / Terminal modes; Trace / Diff are separate tools; default Terminal; plain panes have no agent Chat or Trace | `web/index.html`, `web/app.js` focus/tile, `mobileapp/src/screens/DetailScreen.tsx`; Web order and plain-pane preference fixed |
| Waiting / working / idle / plain process | Needs you / Working / Idle / Running, same core states and status palette | `MenuView.swift`, `CommandPalette.swift`, mobile i18n, Web LABEL; Mac waiting heading and Web status accessible name fixed |
| Group folding | Collapse all / Expand all | `web/app.js`, mobile `PaneBrowserView` and `ChatView`; Web terminology fixed |
| Knowledge and route destinations | Book for knowledge, globe for route | `HQResourcePage.symbol`, mobile `SIcon`, `HQDisc`; already aligned, no icon change |
| Paired-device identity | Stored custom label, platform then address then last seen; unknown information stated | `PairStore.swift`, `ManageMacScreen.tsx`, `/api/devices`; Browser casing, metadata/icon precedence and custom-label cleanup fixed |
| Read-only / input capability | Owner/guest scope enforced by core; read-only UI cannot imply sending | Web composer/input-bar, mobile DetailView, API contract; distinct native controls retained |

The menu bar opens a local pane rather than embedding Chat/Terminal. Web Diff,
workbench tiles and browser-local layout storage are browser-specific capabilities,
not missing phone controls. Pair administration uses the Mac master credential;
phone and iPad show a read-only roster. Those differences remain intentional.

Regression checks exercise English/Chinese tile order, plain-pane capability and
saved-mode retention, custom names, enrollment/reconnect metadata and auth boundaries.
Physical iPhone/iPad and spoken VoiceOver acceptance are not covered by these checks.

## Host platform capabilities

The shared core runs on macOS and Linux; a server is not necessarily a Mac. Use “server” for remote connection, pairing and failure text. Keep Mac/macOS for actual Apple device names and macOS-only controls. `GET /api/host.capabilities` is the support source for desktop focus, desktop notifications, menu bar and lid control; clients must not infer these from agent or terminal names. An explicit false removes the corresponding action. Missing fields retain legacy presentation. Linux headless users manage sessions in the app, browser or terminal; local macOS menu-bar controls remain local.

Service management, host metadata, resource sampling and desktop interaction have separate platform adapters. Keep enrollment, access policy, pane identity and HQ behavior in the shared core. FreeBSD is an extension target, not a currently supported platform.

## Android phones and tablets

Android uses these same Phone and regular-tablet surfaces; it is not a separate business model or a copied UI. `mobileapp/ANDROID.md` records the native boundary and delivery contract. Session/HQ state, Chat, Terminal, Trace, Diff, code formatting, themes, pairing and guest scopes remain shared React Native/Go implementations. Kotlin adapts documents and system sharing, keyboard/haptics, optional FCM and a credential-free last-seen usage widget. The document reader stylesheet is generated from one source for UIKit/WebKit and Android WebView; platform parsers and transport remain native adapters.

Native Android acceptance is independent: bundled QA builds and owned-emulator interactions, phone/tablet resizing, Back/IME, system permissions, TalkBack, physical devices, live FCM and store publication have separate evidence. Missing Google configuration/services must be visible without blocking ordinary paired use. Android does not inherit iOS cloud-backup or Live Activity claims.

## Branded loading parity

App (`LoadingMark` / `BrandLoader`), Web (`brandLoaderEl`) and menu bar
(`BrandProgressMark`) use a fixed three-pane mark. The 100-unit grid has panes
(8,8,40,40), (52,8,40,40), (8,52,84,40), radius9. Cyan (#06B6D4) moves
top-right → bottom → top-left over1.8s; each phase holds for390ms, then
crossfades for210ms. Only highlight opacity changes. Reduce Motion keeps the
top-right pane cyan. Labels describe the read once; decorative panes are hidden
from accessibility. Native views stop their cycle when inactive or unmounted;
browser loaders leave the DOM when their read/view retires. System-owned progress
controls, static identities and the agent-working ring keep their own semantics.

A loading change must inventory these three implementations plus Phone/iPad's
shared component, name the Terminal/attach counterpart or its absence, and compare
actual rendered phases in both themes and compact/wide layouts. Source checks or
a passing build alone do not establish visual parity; accessibility labels do not
claim spoken screen-reader acceptance.

## Component verification

The [current DESIGN rules](DESIGN.md#current-shared-rules-2026-11-17) are the single
shared definition. Each change records actual results, evidence paths and untested
boundaries against this matrix. Listing platform names is not acceptance. Include normal,
loading, empty, failure/retry and disabled states; add queued/saving and waiting/working/idle
where applicable. Check light/dark and Reduce Motion. This table assigns coverage; it does
not claim checks have already run.

| Component / rule | Phone / iPad | Web fine/coarse pointer | Mac | Terminal |
| --- | --- | --- | --- | --- |
| Secondary text TEXT-READ | theme/SettingsRow/AgentRow; contrast, components and native large type | Actual CSS/screenshots, both themes and selection | Theme plus real vibrancy/wallpaper | Terminal-owned palette; no App token assumption |
| Input/attachment TOUCH-44, UPLOAD-PHASE | Composer; independent edit/remove and real byte stages | composer; desktop/390px, XHR stages and retry | Native screenshot-send flow separately | Text/real-byte CLI progress |
| Loading MOTION-LOAD | LoadingMark shared by phone/tablet; lifecycle and Reduce Motion | brandload page/inline/send, Reduce Motion | GtmuxLoadingMark; stop after close; OS exceptions | Text state; no animated logo |
| HQ/session ROLE-HQ, A11Y-CONTEXT | Separate HQ; switches/same-named rows; spoken device check separate | Both layouts retain HQ role; keyboard and labels | Lower-pane status symbol in grayscale; actual material separate | Semantic status/location |
| Documents FILE-SCOPE | Exact scope; unavailable/permission/network; new-link recovery | Generated validation/copy contract | Local document/system preview boundary | Registration followed by recipient-scope read verification |

Project delivery reports use these rule IDs for source binding, actual checks, screenshots,
and remaining device/spoken acceptance gaps.
