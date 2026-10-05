# Web browser mirror: workbench edition (WEB.md)

> The design authority for the browser mirror. Visual reference: [mockup/gtmux-web.dc.html](mockup/gtmux-web.dc.html) (§01 to §04).
> The mockup includes proposals; sections below distinguish them from the current implementation.
> Implementation entry point: `internal/server/web/` (`index.html` / `app.js` / `style.css`). Everything underneath
> reuses the existing contracts (`/api/agents · /api/pane · /api/transcript · /api/diff · /api/icon`); the browser polls view data and fetches icons as needed. Authorized input uses `/api/send` and `/api/upload`.

## Positioning

A browser has a big screen and a real keyboard and mouse. The web mirror must not be a port of the phone's single column layout; it is a desktop workbench. An agent directory grouped by state and window on the left; drag a listed pane onto a board, arrange and resize freely, compose your own view the way you would in tmux. Typing is available only for panes the caller may control.

## Red line: layout stays local; input is authorized

Real tmux can split/kill/spawn; the mirror does not pretend to. "Arranging windows and panes" means the user's own viewing layout; **it never changes the real tmux tree**. Pane input is available only after the server confirms the caller's scope.

**When the shared page can type, a failure has to speak up.** A refused send must explain why and preserve the draft without overwriting a newer draft. `makeComposer` handles ordinary HTTP error bodies under the composer; authentication failures and network failures need separate handling. This is a requirement for the input flow, not a claim that every failure path has been verified.

## 1. Top bar

gtmux logo · layout preset dropdown (Frontend trio…) · snap to grid toggle · Show waiting panes toggle ·
connection indicator (server name + status dot, never the word "live") · appearance (Aa: font/size, reusing the existing settings).

## 2. Left directory (session/window/pane tree)

- The workbench reads `/api/agents`, grouped needs-you→errored→working→idle→running, then by window. Multi-pane windows get a heading; the current headings do not expand/collapse. A search filter is at the top.
- The separate **All panes** browser, reached from the narrow-screen radar, reads `/api/panes` and includes plain shells. It is not the workbench rail.
- In All panes, a plain pane's name comes from the same source as the other two screens (`plainLabel` ↔ macapp `PaneLabels.plain` ↔ mobile
  `api/types.paneLabel`): `title` (skip a whole path or the command itself) → `win_name` (unless tmux auto-renamed it to the command name) →
  `project` → last segment of `cwd` → `command`. Printing only the command makes every shell on a machine read `bash`, which is true but
  distinguishes nothing. **Three implementations, one chain**: a change in one place must be made in all three; the preference order is in DESIGN §16. If every field including command is empty, the phone adds a final pane-ID fallback.
- Drag a rail row onto the board, or double-click it to add a tile. If that pane is already on the board, it is brought forward. Use its ⤢ button for the separate focus view.
- Collapsible: ⇤ in the header folds it away; when collapsed, a small tab with the waiting count (⇥) stays on the board's left edge to reopen it.
- Resizable: a col-resize handle on the right edge, width remembered (localStorage). Below 900px the top-level page uses the single-column radar instead of the workbench.

## 3. Free-form board

- Each tile = a live xterm mirror of one pane (reusing the existing xterm write + scroll lock in `app.js`).
- Tile: drag the title to move, drag the bottom-right corner to resize, stack or tile freely; optional snap to grid alignment.
- Tile header: avatar + corner status badge · name · `terminal / chat / diff` switch · ⤢ full screen · × close.
- A waiting tile gets a red border + a light pulse.
- Multiple panes = multiple concurrent `/api/pane?id` mounts; `diff` uses `/api/diff?id`; `chat` uses `/api/transcript?id`.
- Tiles use absolute positions: resizing one does not reflow the others. Clicking its title without dragging maximizes it within the board; `f` and `1–9` also maximize tiles. Use Restore or Esc to return. This board maximization is distinct from the ⤢ focus view.

## 4. Full-screen focus (single-pane close reading, mockup §02)

A tile’s ⤢ button, a command-palette selection, or a narrow-screen radar row opens the single-pane focus view. It offers Terminal and Chat, previous/next pane, and terminal controls for font size, appearance, copying the selection or visible screen, and jumping to the latest output. Diff remains a workbench tile mode; focus has no diff tab or wrap toggle. Esc returns to the previous top-level view.

**Below 800px (a phone).** The toolbar does not fit one row, so the bar wraps: back, title and server on the first row (the title truncates first), the identity and input chips and the controls on the rows below, as many as the width needs. The layout is intended to fit the viewport; the appearance panel opens under the bar. The 390px and 1440px English/Chinese owner/guest fixture checks cover those sizes, not every possible device or content length.

**Codex's pinned prompt.** Codex pins the prompt of the turn on screen to row 0, cut at the pane's width with "…". In the single-pane terminal view the full prompt from the conversation log takes a bar above the terminal (two lines at rest, a click opens it, Copy) and the cut row leaves the terminal. The rules are the phone's (MOBILE.md), run by a JavaScript copy in `app.js` against the same case file (`mobileapp/src/ui/codexPinnedCases.json`) and the same tmux-measured cell widths, to check agreement on those cases and width tables. While the row is on screen but unexplained, the view fetches the log at most every 4 s. Not in workbench tiles: there is no room for a second bar in a tile, and its row stays as captured.

## 5. Chat mode · wide-screen edition (mockup §03)

Same source as the mobile `ChatView` (`/api/transcript`: prompt → collapsed intermediate steps → agent reply), re-laid-out for wide screens:

- Turn directory on the left: lists every turn, `j`/`k` to jump, current turn highlighted (global navigation only the big screen has).
- Centered chat column (up to 820px including padding): user bubbles on the right with the human avatar; agent bubbles on the left with the official icon; hovering a bubble reveals "copy / quote" (a desktop-mouse feature).
- Waiting card: `/api/options` supplies parsed numbers and labels. A caller authorized to type can click a choice to send its digit via `/api/send`, without Enter; read-only callers see the choices without send handlers. Empty results show a terminal-answer hint instead of chat choice buttons. Waiting can be an open question, not necessarily an approval.
- Intermediate steps can be expanded/collapsed. Free-text input is in **Terminal**, not Chat: its multiline composer uses Enter to send and Shift/Option+Enter for a newline, with image upload and a control-key strip. The chat surface has a dark background. Copy and Quote copy text to the clipboard; Quote does not submit it.

## 6. Your avatar · the human in the agent era (mockup §03 appendix)

The current browser uses a fixed person-battery SVG on a cyan gradient disc for an unattributed human prompt. A turn with sender attribution uses the HQ wordmark or the sending agent’s avatar instead (`senderAvatarEl`). The color does not identify a person.

The mockup’s other variants (Ascension / Conductor / Decider / Captain / Resting / Rubber stamp / Dog-walk reversal / Hamster wheel), photo upload, emoji and initials are **proposals, not implemented browser settings**. They do not establish that all three surfaces have an avatar picker.

## 7. Delivered layout capabilities

- Named presets store pane IDs, positions, sizes, modes, rail state and snap preference in localStorage; the top bar saves/applies/deletes them. The current board is also saved. Restoration includes only panes found in the current agent response; browser storage and origin determine which saved layouts are available.
- Show waiting panes (optional): the two-second agent poll detects a transition into waiting and adds a missing tile with a pulse. The first poll is skipped, and existing tiles are not added again. This browser behavior is polling-based, not an SSE alert handler.
- Board maximization and the separate single-pane focus view are both implemented; see §§3–4 for their different entry points and modes.

## 8. Keyboard (desktop first)

In the workbench, outside a text field: `⌘K` / `Ctrl+K` opens the pane palette; `1–9` maximizes the Nth tile; `f` toggles board maximization of the front tile; `Esc` restores it; `g` toggles snap; `[ ]` cycles saved presets; `/` focuses directory search. In focused Chat, `j/k` moves through turns and `c` collapses steps; in focused Terminal, `j/k` moves between panes. These are page shortcuts, not raw terminal keystroke passthrough.

## 9. State / landing

- The radar uses the shared color/shape/glyph vocabulary. The connection dot changes on polling failures. A failed tile fetch currently retains its previous content; there is no implemented per-tile offline gray overlay, despite the unused CSS class.
- `web/app.js` already contains the board layout engine (absolute positioning, drag/resize, localStorage), presets, waiting-pane surfacing and focus views. Below 900px, the top-level view becomes radar→pane.
- The original staged rollout is complete for these layout features. It does not include the avatar picker or HQ command deck proposed below.


---

## 10. Browser language and proposed HQ command deck · §07 mockup

### Language: follow the browser

The page follows the browser's language, because the browser mirror is the only screen you hand to someone else. The recipient of a share link is a guest, exactly the person least likely
to read the host's language, and this page had drifted into "Chinese UI + a few bilingual spots" (the gate screen and the connection-status
hints were bilingual, the rest was not), so an English reader opening a guest link saw a page they could not operate (fixed 2026-09-07).

It chooses by the first `navigator.languages` entry, falling back to `navigator.language`, the same reasoning as the phone following the device language; there is no `GTMUX_LANG` to read here,
and the person opening it may not have gtmux installed at all. What is hard-coded in `index.html` is the Chinese half; `app.js` relabels
at boot using `CHROME` and other `labelChrome` assignments. Dynamic labels use `T(en, zh)` or language branches; not every visible string is in one table.

**The gate screen is the exception and keeps showing both languages**: that is the screen you screenshot and send to whoever can fix it.

The access page serves both Mac owners and share-link guests: it accepts a pairing or share code, and an invalid-code message asks for a checked or new code without assuming a sender. An empty chat screen points to Terminal without attributing the agent transcript to hooks. A read-only pane states directly that input is not allowed.


`internal/server/webui_test.go` checks an explicit list of control IDs and scans for Chinese literals without a nearby language switch. This is a source guard, not a proof that every control is translated or that each translation is accurate.

English capitalization of labels/buttons follows the three-tier rule in DESIGN §11 / MOBILE §6 (names and actions sentence-cased, state
words all lowercase, key names all uppercase small text); this page copies it as is when landing, with no local variants, because local
variants are exactly how the phone drifted into "casing is arbitrary".

**Proposal, not a delivered Web route:** three columns for fleet situation (`/api/digest`), conversation with HQ, and a dispatch ledger (`/api/tasks`, spawn/reap), with an owner-only gate and a narrow-screen stack. The current Web page has no HQ command deck or these API calls; HQ is represented within the existing pane/radar and sender-attribution views. The proposed special deck, pinning and breakpoint must not be advertised as shipped merely because the backend has these endpoints.

## 11. Input capability + permission surfacing · §08 mockup

The Web page sends through `POST /api/send`; it does not use the CLI’s raw `/api/attach` WebSocket. Every focused pane and tile states its input capability once `/api/share` has resolved. An authorized **Terminal** view has a composer; a read-only view has a note instead of an input area. Parsed choices in focused Chat may also be clicked by authorized callers.

Owners can type into panes their owner credential controls. A guest needs both the share link’s input scope (input ⊆ visible) and the host’s “allow collaborators to type” switch. The top bar distinguishes owner and guest. The server checks authorization on requests; after revocation, new requests with that credential are rejected. This HTTP statement is not a claim about the separate CLI attach stream’s scope or shutdown timing.
