# Web browser mirror — workbench edition (WEB.md)

> The design authority for the browser mirror. Visual reference: `mockup/gtmux-web.dc.html` (§01–§04).
> Implementation entry point: `internal/server/web/` (`index.html` / `app.js` / `style.css`). Everything underneath
> reuses the existing contracts (`/api/agents · /api/pane · /api/transcript · /api/diff · /api/icon · SSE`); **no new backend**.

## Positioning

A browser has a big screen and a real keyboard and mouse, so this is **not a copy of the phone's single column**; it is a **desktop workbench**. A session/window/pane directory on the left; drag any pane onto a board, arrange and resize freely, compose your own view the way you would in tmux. **Still a read-only mirror.**

## Red line: read-only

Real tmux can split/kill/spawn; the mirror **does not pretend to**. "Arranging windows and panes" means the user's **own viewing layout**; it never changes the real tmux tree. Input still goes back to the phone or the Mac.

**When the shared page can type, a failure has to speak up**: `/api/send` refuses for concrete reasons (someone is typing in that pane / the session is gone / the input box did not confirm), and this screen used to swallow all of them silently, and worse, it **cleared the composer before the result came back**, so the message was gone, the typed text was gone, and the screen said nothing. Now: the reason is shown verbatim under the composer (amber, the same tier as the errored group), and on failure the text is put back (only if the composer is still empty, since the reader may already be typing the next line). The phone follows the same rule.

## 1. Top bar

gtmux logo · **layout preset** dropdown (Frontend trio…) · **snap to grid** toggle · **auto-surface waiting** toggle ·
connection indicator (server name + status dot, never the word "live") · appearance (Aa: font/size, reusing the existing settings).

## 2. Left directory (session/window/pane tree)

- Grouped by state needs-you→working→idle; a window expands to its panes; a search filter at the top.
- **A plain pane's name comes from the same source as the other two screens** (`plainLabel` ↔ macapp `PaneLabels.plain` ↔ mobile
  `api/types.paneLabel`): `title` (a whole path does not count as a name) → `win_name` (unless tmux auto-renamed it to the command name) →
  `project` → last segment of `cwd` → `command`. Printing only the command makes every shell on a machine read `bash`, which is true but
  distinguishes nothing. **Three implementations, one chain**: a change in one place must be made in all three; the full rule is in DESIGN §16.
- **Drag a pane → board**; double-click → full screen.
- **Collapsible**: ⇤ in the header folds it away; when collapsed, a small tab with the waiting count (⇥) stays on the board's left edge to reopen it.
- **Resizable**: a col-resize handle on the right edge, width remembered (localStorage). Narrow screens (<900px) collapse it automatically.

## 3. Free-form board

- Each tile = a **live xterm mirror** of one pane (reusing the existing xterm write + scroll lock in `app.js`).
- Tile: drag the title to move, drag the bottom-right corner to resize, stack or tile freely; optional **snap to grid** alignment.
- Tile header: avatar + corner status badge · name · `terminal / chat / diff` switch · ⤢ full screen · × close.
- **A waiting tile** gets a red border + a light pulse.
- Multiple panes = multiple concurrent `/api/pane?id` mounts; `diff` uses `/api/diff?id`; `chat` uses `/api/transcript?id`.
- **Resizing one tile reflows the rest** (grid/flex layout when not in free-stack mode); **a single click on a tile maximizes it**, click again or Esc to restore.

## 4. Full-screen focus (single-pane close reading, mockup §02)

Double-click a tile / ⤢ / single-click maximize → one pane fills the screen, giving the largest reading area + the complete toolbar (terminal/chat/diff, A−/A+, wrap/scroll, copy visible screen/scrollback, jump to latest). Esc returns to the board.

## 5. Chat mode · wide-screen edition (mockup §03)

Same source as the mobile `ChatView` (`/api/transcript`: prompt → collapsed intermediate steps → agent reply), re-laid-out for wide screens:
- **Turn directory on the left**: lists every turn, `j`/`k` to jump, current turn highlighted (global navigation only the big screen has).
- **Centered chat column** (~680px readable width): user bubbles on the right with the human avatar; agent bubbles on the left with the official icon; **hovering a bubble reveals "copy / quote"** (a desktop-mouse feature).
- **Approval card**: while waiting, full-width large buttons `1/2/3` (real labels); one click sends via `/api/send`, same source as the menu bar and notifications.
- Collapsed steps; multi-line composer at the bottom (⏎ send, ⌥⏎/⤓ newline). The chat surface is always dark.

## 6. Your avatar · the human in the agent era (mockup §03 appendix)

The human's avatar in the chat. Default is the **human battery** (a person inside a battery, powering it: you think you are using it, it is feeding on you), on a uniform cyan gradient ground, consistent with the brand and distinct from agent avatars (agents use official icons/squares, humans a gradient circle). Settings offer the other variants (Ascension / Conductor / Decider / Captain / Resting / Rubber stamp / Dog-walk reversal / Hamster wheel), or upload a photo / pick an emoji / use initials. The color is only the brand color and encodes no identity. Replaces the existing `UserAvatar` on all three screens.

## 7. Proposed new capabilities

- **Save layouts/presets**: named layouts (which panes, positions, sizes) stored in localStorage, switched from the top bar; reopening the link restores them.
- **Auto-surface waiting** (optional): on an SSE `alert kind:"waiting"`, that pane comes onto the board and pulses → the board becomes the radar.
- **Focus mode**: double-click a tile / ⤢ → full-screen single pane (close reading / scroll history / view diff); Esc returns to the board (equivalent to the existing single-column view).

## 8. Keyboard (desktop first)

`⌘K` command palette to summon a pane · `1–9` focus the Nth tile · `f` full screen · `Esc` leave full screen · `g` snap to grid ·
`[ ]` cycle layout presets · `/` search the directory.

## 9. State / landing

- The state language matches all three screens (color + shape + glyph); an offline tile is grayed out, never cleared.
- Implementation: add a **board layout engine** to `web/app.js` (absolute positioning + drag/resize + localStorage persistence),
  tiles reusing the existing xterm logic; responsive fallback: narrow screens return to the existing single-column radar→pane.
- Incremental: first concurrent multi-pane + drag/resize + collapse/resize, then presets / auto-surface / focus.


---

## 10. HQ supervisor wide-screen command deck · §07 mockup

### Language: follow the browser

The browser mirror is **the only screen you hand to someone else**. The recipient of a share link is a guest, exactly the person least likely
to read the host's language, and this page had drifted into "Chinese UI + a few bilingual spots" (the gate screen and the connection-status
hints were bilingual, the rest was not), so an English reader opening a guest link saw a page they could not operate (fixed 2026-09-07).

It chooses by **`navigator.language`**, the same reasoning as the phone following the device language; there is no `GTMUX_LANG` to read here,
and the person opening it may not have gtmux installed at all. What is hard-coded in `index.html` is **the Chinese half**; `app.js` relabels
at boot from the `CHROME` table, so every string a reader can see lives in one table instead of being scattered across `data-en` attributes.

**The gate screen is the exception and keeps showing both languages**: that is the screen you screenshot and send to whoever can fix it.

`internal/server/webui_test.go` pins this: a control with an id in the page that `app.js` never relabels is a red build.

**English capitalization of labels/buttons follows the three-tier rule in DESIGN §11 / MOBILE §6** (names and actions sentence-cased, state
words all lowercase, key names all uppercase small text); this page copies it as is when landing, no local variants. That is exactly how the
phone drifted into "casing is arbitrary".

Three wide-screen columns: left **fleet situation** (`/api/digest`) · center **conversation with HQ** · right **dispatch ledger** (`/api/tasks`, spawn/reap). In the directory tree HQ is pinned to the top with ⌂; clicking it opens the command deck rather than a plain pane mirror; commands go to the HQ pane via `/api/send`. Narrow screens (<1100px) collapse to a vertical stack. The HQ command deck is **not open to guests**.

## 11. Input capability + permission surfacing · §08 mockup

The web page can type (`POST /api/send` / `attach`). Every tile header states **⌨ can type** (cyan, with composer + 1/2/3) or **👁 read-only** (gray, no input area + a "not authorized" note, never an empty text box). The owner can type into every pane; a guest only into the "input" scope ticked on that share link (per-link scope, input ⊆ visible) + the host's "allow collaborators to type" master switch. Owner and guest see different identities in the top bar. Enforced server-side, revocation immediate.
