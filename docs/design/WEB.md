# Web browser mirror: workbench edition (WEB.md)

> The design authority for the browser mirror. Visual reference: [mockup/gtmux-web.dc.html](mockup/gtmux-web.dc.html) (§01 to §04).
> The mockup includes proposals; sections below distinguish them from the current implementation.
> Implementation entry point: `internal/server/web/` (`index.html` / `app.js` / `style.css`). Everything underneath
> reuses the existing contracts (`/api/agents · /api/pane · /api/transcript · /api/diff · /api/icon`); the browser polls view data and fetches icons as needed. Authorized input uses `/api/send` and `/api/upload`; permissions and choices use `/api/share` and `/api/options`. The other panes, appearance and enrollment flows use `/api/panes`, `/api/theme` and `/api/enroll`.

Uploads use multipart `file` with `POST /api/upload?id=<pane_id>`. Guest links require host input consent, current grants, and that link's input permission for the target pane, just like `/api/send`. Owner/device callers retain support for uploads without `id`.

## Positioning

A browser has a big screen and a real keyboard and mouse. The web mirror must not be a port of the phone's single column layout; it is a desktop workbench. An agent directory grouped by state and window on the left; drag a listed pane onto a board, arrange and resize freely, compose your own view the way you would in tmux. Typing is available only for panes the caller may control.

## Red line: layout stays local; input is authorized

Real tmux can split/kill/spawn; the mirror does not pretend to. "Arranging windows and panes" means the user's own viewing layout; **it never changes the real tmux tree**. Pane input is available only after the server confirms the caller's scope.

**When the shared page can type, a failure has to speak up.** A refused send must explain why and preserve the draft without overwriting a newer draft. `makeComposer` handles ordinary HTTP error bodies under the composer. The 403, 409 and network-failure paths restore the submitted text only while the box is still empty; a newer draft stays in place ([#1391](https://github.com/chenchaoyi/gtmux/pull/1391)). A network failure means delivery is unconfirmed, so the page must not claim the text was definitely not sent or resend it automatically.

Enter confirms an active input-method composition without sending it. Plain Enter outside composition sends; Shift/Option+Enter inserts a newline.

## 1. Top bar

gtmux logo · layout preset dropdown (Frontend trio…) · snap to grid toggle · Show waiting panes toggle ·
connection indicator (server name + status dot, never the word "live") · appearance (Aa: font/size, reusing the existing settings).

## 2. Left directory (session/window/pane tree)

- The workbench reads `/api/agents`, grouped needs-you→errored→working→idle→running, then by window. Multi-pane windows get a heading; the current headings do not expand/collapse. A search filter is at the top.
- Native sessions appear separately as **Elsewhere**, with a **Status only** label. They cannot be dragged onto the board, added by double-click, selected in the pane palette or reached by previous/next pane navigation. These controls require a real tmux pane ID, matching the narrow radar's boundary.
- The separate **All panes** browser, reached from the narrow-screen radar (or `p` on a keyboard), reads `/api/panes` and includes plain shells. It is not the workbench rail.
- All panes (2026-10-06): until the first read lands it shows the brand loader with "Reading the panes on this Mac…" and the count line says "reading…"; a failed first read says it could not read and is trying again, and a failed refresh keeps the rows and adds " · not refreshed". Agent rows carry their real status badge from `/api/agents`, which the browser reads alongside `/api/panes` (the radar's own poll stops while it is open); the "on radar" tag is gone. Each session header shows its pane and agent counts plus a badge and count per non-zero status (waiting in red), and the count line adds " · N need you". Clicking a header folds the session; "Collapse all"/"Expand all" sits in the bar; the folds are kept in this browser's `localStorage` (`gtmux.panes.folded`); a search shows matches inside a folded session. Widening the window past the workbench width keeps All panes on screen alone.
- In All panes, a plain pane's name comes from the same source as the other two screens (`plainLabel` ↔ macapp `PaneLabels.plain` ↔ mobile
  `api/types.paneLabel`): `title` (skip a whole path or the command itself) → `win_name` (unless tmux auto-renamed it to the command name) →
  `project` → last segment of `cwd` → `command`. Printing only the command makes every shell on a machine read `bash`, which is true but
  distinguishes nothing. **Three implementations, one chain**: a change in one place must be made in all three; the preference order is in DESIGN §16. If every field including command is empty, the phone adds a final pane-ID fallback.
- Drag a rail row onto the board, or double-click it to add a tile. If that pane is already on the board, it is brought forward. Use its ⤢ button for the separate focus view.
- Collapsible: ⇤ in the header folds it away; when collapsed, a small tab with the waiting count (⇥) stays on the board's left edge to reopen it.
- The tile stack is isolated within the board, so it cannot cover the sidebar tab or other surrounding controls. A maximized tile also remains below its Restore control after repeated focus clicks.
- Resizable: a col-resize handle on the right edge, width remembered (localStorage). Below 900px the top-level page uses the single-column radar instead of the workbench.

### Historical native launch host

A valid optional `launch_host` displays the harness and observed host (for example,
`Codex · sample-client`). An unconfirmed native client without host evidence displays
`Codex · External session`. Conversation details show the host observation time and
retain “Current input client unconfirmed”; the host never changes action permissions.
All detection belongs to the Go core, with no consumer-side ancestry lookup. See
[agent integrations](../guides/agent-integrations.md#where-an-external-conversation-was-launched)
for extraction and privacy limits. The observation can be old; it is not current ownership.

## 3. Free-form board

- Each tile = a live xterm mirror of one pane (reusing the existing xterm write + scroll lock in `app.js`).
- Tile: drag the title to move, drag the bottom-right corner to resize, stack or tile freely; optional snap to grid alignment.
- Tile header: avatar + corner status badge · name · `Chat / Terminal / Diff` switch · ⤢ full screen · × close.
- A waiting tile gets a red border + a light pulse.
- Multiple panes = multiple concurrent `/api/pane?id` mounts; `diff` uses `/api/diff?id`; `chat` uses `/api/transcript?id`.
- Tiles use absolute positions: resizing one does not reflow the others. Clicking anywhere in its header except a button, without dragging, maximizes it within the board; `f` and `1–9` also maximize tiles. The workbench-only command palette adds or finds a tile, flashes it, and maximizes it in the board. Use Restore or Esc to return. This board maximization is distinct from the ⤢ focus view.

Tile headers keep mode labels on one line and wrap the control group when the tile is narrow; the input-capability chip remains visible.

## 4. Full-screen focus (single-pane close reading, mockup §02)

A tile’s ⤢ button or a narrow-screen radar row opens the single-pane focus view. For agents it offers Chat then Terminal, previous/next pane, and terminal controls for font size, appearance, copying the selection or visible screen, and jumping to the latest output. Diff remains a workbench tile mode; focus has no diff tab or wrap toggle. Esc returns to the previous top-level view.

**Below 800px (a phone).** The toolbar does not fit one row, so the bar wraps: back, title and server on the first row (the title truncates first), the identity and input chips and the controls on the rows below, as many as the width needs. The layout is intended to fit the viewport; the appearance panel opens under the bar.
This also applies to plain shell panes, whose Chat/Terminal tabs are hidden.

**Codex's pinned prompt.** Codex pins the prompt of the turn on screen to row 0, cut at the pane's width with "…". In the single-pane terminal view the full prompt from the conversation log takes a bar above the terminal (two lines at rest, a click opens it, Copy) and the cut row leaves the terminal. The rules are the phone's (MOBILE.md), run by a JavaScript copy in `app.js` against the same case file (`mobileapp/src/ui/codexPinnedCases.json`) and the same tmux-measured cell widths, to check agreement on those cases and width tables. While the row is on screen but unexplained, the view fetches the log at most every 4 s. Not in workbench tiles: there is no room for a second bar in a tile, and its row stays as captured.

## 5. Chat mode · wide-screen edition (mockup §03)

Same source as the mobile `ChatView` (`/api/transcript`: prompt → collapsed intermediate steps → agent reply), re-laid-out for wide screens:

- Turn directory on the left: lists every turn, `j`/`k` to jump, current turn highlighted (global navigation only the big screen has).
- Centered chat column (up to 820px including padding): user bubbles on the right with the human avatar; agent bubbles on the left with the official icon; Copy and Copy as quote are icon-only actions with accessible names and tooltips. Pointer hover or keyboard focus reveals them below the text; touch keeps them visible.
- Waiting card: `/api/options` supplies parsed numbers and labels. A caller authorized to type can click a choice to send its digit via `/api/send`, without Enter; read-only callers see the choices without send handlers. Only parsed options become buttons: empty results or a failed options request clear the choices, never invent `1/2/3`. A typable chat view says to answer in Terminal; a read-only view says to use the phone/Mac. Waiting can be an open question, not necessarily an approval.
- Intermediate steps can be expanded/collapsed. Free-text input is in **Terminal**, not Chat: its multiline composer uses Enter to send and Shift/Option+Enter for a newline, with image upload and a control-key strip. The chat surface has a dark background. Copy and Copy as quote copy text to the clipboard; quoting does not submit it. All copy paths use the same secure Clipboard API with a plain-HTTP/denied-permission fallback, restore focus/selection, and report success only after copying succeeds.
- Refresh compares the complete returned turns, including segment text and tool details, in focus and tiles. An unchanged poll preserves the view; updates keep the existing step expansion and scroll position in focus.

## 6. Your avatar · the human in the agent era (mockup §03 appendix)

The current browser uses a fixed person-battery SVG on a cyan gradient disc for an unattributed human prompt. A turn with sender attribution uses the HQ wordmark or the sending agent’s avatar instead (`senderAvatarEl`). The color does not identify a person.

The mockup’s other variants (Ascension / Conductor / Decider / Captain / Resting / Rubber stamp / Dog-walk reversal / Hamster wheel), photo upload, emoji and initials are **proposals, not implemented browser settings**. They do not establish that all three surfaces have an avatar picker.

## 7. Delivered layout capabilities

- Named presets store pane IDs, positions, sizes, modes, rail state and snap preference in localStorage; the top bar saves/applies/deletes them. The current board is also saved. Restoration includes only panes found in the current agent response; browser storage and origin determine which saved layouts are available.
- Show waiting panes (optional): the two-second agent poll detects a transition into waiting and adds a missing tile with a pulse. The first poll is skipped, and existing tiles are not added again. This browser behavior is polling-based, not an SSE alert handler.
- Board maximization and the separate single-pane focus view are both implemented; see §§3–4 for their different entry points and modes.

## 8. Keyboard (desktop first)

In the workbench, outside a text field: `⌘K` / `Ctrl+K` opens the pane palette; `1–9` maximizes the Nth tile; `f` toggles board maximization of the front tile; `Esc` restores it; `g` toggles snap; `[ ]` cycles saved presets; `/` focuses directory search. In focused Chat, `j/k` moves through turns and `c` collapses steps; in focused Terminal, `j/k` moves between panes. These are page shortcuts, not raw terminal keystroke passthrough.

Page shortcuts apply only outside buttons, links and editable regions, and ignore composition or already-handled events. Browser/OS modifier combinations retain their native behavior; only the workbench’s explicit `⌘K` / `Ctrl+K` palette shortcut is intercepted. A focused button keeps Enter/Space even when the radar has a selected row.

## 9. State / landing

- The radar uses the shared color/shape/glyph vocabulary. The connection dot changes on polling failures. A failed tile fetch currently retains its previous content; there is no implemented per-tile offline gray overlay, despite the unused CSS class.
- `web/app.js` already contains the board layout engine (absolute positioning, drag/resize, localStorage), presets, waiting-pane surfacing and focus views. Below 900px, the top-level view becomes radar→pane.
- The original staged rollout is complete for these layout features. It does not include the avatar picker proposed in §6 or the HQ command deck proposed in §10.
- Focus and roster reads belong to the view and credential that requested them. Switching pane or mode, going home or losing access invalidates earlier replies, including replies whose headers arrived before the switch. A newer completed read supersedes an older one; merely starting another poll does not discard a slow valid response.
- On an authentication failure, the access page clears the old title, identity and permission labels and stops focus, roster, options and tile polling. Tile contents are discarded while their local layout is retained; re-pairing restores only panes in the new authorized roster. The code field can be used again.


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

**Proposal, not a delivered Web route:** three columns: fleet situation on the left (`/api/digest`), conversation with HQ in the center, and a dispatch ledger on the right (`/api/tasks`, spawn/reap), with an owner-only gate and a narrow-screen stack. The current Web page has no HQ command deck or these API calls; HQ is represented within the existing pane/radar and sender-attribution views. The proposed special deck, pinning and breakpoint must not be advertised as shipped merely because the backend has these endpoints.

## 11. Input capability + permission surfacing · §08 mockup

The Web page sends through `POST /api/send`; it does not use the CLI’s raw `/api/attach` WebSocket. Every focused pane and tile states its input capability once `/api/share` has resolved. An authorized **Terminal** view has a composer; a read-only view has a note instead of an input area. Parsed choices in focused Chat may also be clicked by authorized callers.

Owners can type into panes their owner credential controls. A guest needs the share link’s input scope (input ⊆ visible), the host’s “allow collaborators to type” switch, and grants that are still current (`GrantsStale` is false). The top bar distinguishes owner and guest. The server checks authorization on requests; after revocation, new requests with that credential are rejected. This HTTP statement is not a claim about the separate CLI attach stream’s scope or shutdown timing.

## Browser entry under a Direct path

A share URL may end at `/p<port>#code=...`, with no trailing slash. The HTML establishes a same-origin document base before loading any stylesheet or script. Static resources and API requests use the same Mac prefix; root and explicit `index.html` entries are supported too. Query strings and credential fragments never enter the asset base. Existing share links do not need replacement, and guest access remains governed by server scope checks.

### Shared mode semantics (2026-10-10)

Focus and workbench order Chat before Terminal, matching the phone/iPad controls.
Plain panes offer Terminal only (plus workbench-only Diff). Opening a plain pane
neither exposes an empty Chat view nor overwrites the user's saved agent mode. The
initial default remains Terminal. Visible status and its accessible name use the
same LABEL mapping; working is not announced as a generic running process. Generic
pairing names use Browser. Browser metadata stays coarse and best-effort.

Plain panes retain the same `plainLabel` title when opened in focus; their subtitle does not start with an empty agent separator.

### Transient send receipts
The composer briefly confirms accepted text delivery to a working agent without promising when it is consumed. This informational note expires after six seconds. Starting another send cancels its timer; a later refusal or uncertain-send warning remains visible with recoverable text. Control keys do not create a message-waiting note. This follows the phone/iPad receipt wording and expiry policy.


## Terminal Keys panel (2026-10-19)

The terminal composer has one visible **Keys** entry in the frequent-key strip, with
Navigation, Function (F1–F12) and Characters. Existing frequent controls retain their
order and use the shared server catalog when available. Ctrl/Alt/Shift affect the next
explicit tap only; they clear on tap, dismissal, target change, backgrounding or input
permission loss. Unsupported combinations are disabled with an accessible reason.
The panel preserves drafts and attachments and does not append Return. Selecting
Return or Ctrl+M deliberately sends Return; the Characters panel explains aliases.

The phone and iPad use the same component, with a 480pt cap, safe-area/height limits,
scrolling and at least 44pt buttons. It takes no global hardware shortcuts or IME input.
The browser uses the same catalog and a scoped, keyboard-accessible dialog. There are
no macros, modifier locking, repeats or offline queues. Old servers retain legacy keys.

Only input-authorized live tmux routes can send. A fresh Codex collapsed-question hint
may add **View question** without changing `working` to another status. Unknown hints
suppress that action. A receipt says **Sent to terminal**, never that the agent acted.
An uncertain result pauses further sends; an explicit retry reuses the exact original
ID/context. Changed/expired context cannot replay it. See the [HTTP contract](../../api/contract.md#typed-keyaction-forms-of-post-apisend)
and [research and acceptance boundaries](terminal-keyboard-research.md).

### Transcript activity cards (2026-10-19)

Focused Chat uses the same Activity hierarchy, operation labels and neutral action icons as phone/iPad. A heading shows the call count once; folded summaries count action types. Each row offers a short preview, then a native disclosure for the recorded tool name, working directory, input and source-truncation notice when available. Delegation previews show the recorded purpose; wait previews label explicit millisecond settings as settings, never elapsed time or completion. Unknown or partial parameters remain inspectable without guessing. Original input is plain text in a focusable, bounded, horizontally scrollable region with Copy. The group disclosure is a labelled button with `aria-expanded`, at least 44px tall. Group and row expansion, keyboard focus and input scroll survive live updates. Collapse all is a borderless brand-colored action beside the conversation status. `activity.js` holds presentation rules; tests pin its label, alias and icon tables against the mobile source. Compact workbench tiles retain reply-only rendering. [Sources and limits](transcript-activity-research.md).
