# Design — ipad-universal-app

Each decision names what it rejected and why, so the next iteration reads the record
instead of re-deriving it. Canvas: https://claude.ai/code/artifact/b4c7d610-88cb-4b2a-b58f-18dbe789b914

## D1 — One universal target, not an iPad app

`TARGETED_DEVICE_FAMILY = "1,2"` on the app, the widget and the notification service.
One bundle, one listing (`id6791144062`), one review. Rejected: a separate iPad target
or bundle — two builds, two screenshot sets to keep, two What's New streams, and a user
who owns both devices buying twice.

## D2 — Two size classes, one rule, and a collapsible sidebar instead of a drawer

`useSizeClass()` returns `regular` when `isSplitCanvas(width, height)` (≥ 768 × ≥ 600,
unchanged) and `compact` otherwise. Regular = sidebar + main pane; compact = the phone.

§5 as written in 2026-07 had a third state: portrait or a narrowed Split View turns the
sidebar into a ☰ drawer. Rejected. An 11" iPad in portrait is 834 wide; a 280pt sidebar
leaves 554 for the main pane, wider than any phone (393–430). A drawer is a third layout
to keep true, with its own open/close state, gesture and animation, to solve a problem
the numbers do not show. What the drawer was really for — "give me the whole width for a
minute" — is a **collapse toggle** on the sidebar (persisted; ⌃⌘S), which also covers
Stage Manager windows sized between 768 and 1000.

Sidebar width: 300 at ≥ 1000, 280 below. iPad 1/2 Split View (683) and Slide Over (320)
are compact on purpose, as `layout.test.ts` already pins.

## D3 — Composition, not copies (the drift rule)

The phone radar and the sidebar are ONE component, `RadarPanel`: header (brand, server
chip, connection dot, summary), banners, `SectionList`, the HQ entry, the footer. The
two callers differ only in props: `onSelect` (push a route / select in place),
`selectedId` (the accent bar), `width`, and which HQ entry (`HQCard` in the sidebar,
`HQDisc` floating on the phone). `SplitScreen.tsx` is deleted.

Detail is already shared (`DetailView`). `HQScreen` and `PaneBrowserScreen` split into a
route wrapper (compact: back button, safe area) and a view (both shells), the view
taking `layout: 'compact' | 'regular'`.

Enforced: `src/screens/shellDrift.test.ts` reads the source and fails if (a) a second
file renders `SectionList` outside `RadarPanel`, (b) `SplitScreen` reappears, (c) a
screen imports `useWindowDimensions` to pick a layout instead of `useSizeClass`. Same
shape as `detailChrome.test.ts`; what regressed before was structural and no render
test saw it.

## D4 — Selection is workspace state

`WorkspaceContext` holds `selection: {kind:'pane', id} | {kind:'hq'} | {kind:'panes'} |
null` and `select()`. The compact shell turns a selection into `navigation.navigate`
(as `RadarScreen` does today); the regular shell renders it in the main pane. Push deep
links, the HQ page's "open session", keyboard shortcuts and the pane browser all call
`select()` and never touch a navigator. Rejected: route params only (the main pane is
not a route) and `SplitScreen`'s local `selectedId` (lost on remount, invisible to a key
command, unreachable from a push).

## D5 — HQ page on a wide canvas: console + inspector (direction A)

The report header (verdict, standing, the three doors) spans the main pane. Below it the
console takes the width (capped, D6) and a 360pt inspector on the right carries the two
zones the phone puts behind tabs: "Your call" (decision cards, then what is running) and
"HQ's work". Chips and the composer stay at the bottom of the console.

Alternatives on the canvas's second page: **B** stacks both zones above the console
(one glance, but the console loses ~150pt on every screen and the two zones scroll
together); **C** keeps the phone's tabs, wider (nothing new to keep true, but a 1194pt
screen shows one zone and the calls hide behind a tab exactly as on the phone). A was
chosen because the calls are what the page exists for, and beside the console they are
visible while you type the answer.

## D6 — Reading width

Chat, the HQ console, settings and the sheets' text cap at 760pt and centre. The
terminal never caps: more columns is the point of the large screen (`NativeTerm` already
derives columns from the window). `ContentColumn` (600, settings) stays as it is.

## D7 — Hardware keyboard through one native bridge and one table

RN on iOS has no hardware-key API. A small native module (`KeyCommands.swift`) registers
`UIKeyCommand`s on the root view controller from a list JS sends, and emits the command
id on press; iPadOS then draws the ⌘-hold HUD from the titles for free. JS keeps ONE
table, `src/keys/keymap.ts` (id, keys, title en/zh, action), consumed by the bridge, by
the ⌘ HUD (titles) and by a test that every action exists. Bindings: ↑↓ move the radar
selection, ⏎ open, ⌘1–9 jump, ⌘⇧H HQ, ⌘⇧P All panes, ⌘F search panes, ⌘K focus the
composer, esc close a sheet, ⌘[ ⌘] chat/terminal, ⌘+ ⌘− font. Rejected: JS `onKeyPress`
on a hidden TextInput (only sees keys while it has focus, no modifiers).

## D8 — Pointer

`Pressable` rows and buttons take a hover tint (`rowSelected`) via `onHoverIn/Out`.
No custom cursors, no right-click menus in this change (long-press sheets already
exist; secondary click maps to them on iPadOS by default for a long-press).

## D9 — Multitasking and orientation

No `UIRequiresFullScreen`: Split View, Slide Over and Stage Manager are allowed, and the
shell follows the window (D2). All four iPad orientations, nothing locked. Multi-window
(a scene manifest, several radars) is out of scope: RN 0.86's AppDelegate is not
scene-based and nothing in the product needs two windows yet. Apple Pencil annotation
(ITERATIONS D10) stays deferred.

## D10 — Store assets from the same pipeline

The 13" slot (2064×2752) is drawn by `appstore-shots.test.ts` in demo mode on the iPad
Pro 13" simulator, framed by `frame-shots.mjs` parameterised by slot (it hardcodes the
6.9" phone today). Captions come from the same `shot-captions.json` with an `ipad`
set. The lock-screen shot is phone-only (Live Activity has no iPad presentation).

## D11 — Sheets on regular

Board and Usage stay `pageSheet` (iPadOS centres them as form sheets). Knowledge
becomes list | entry inside the sheet on regular, the menu-bar's layout (MOBILE §4
"菜单栏的知识库：列表与正文并排"), with the same `KnowledgeSheet` and a layout prop.
