# Web browser mirror: workbench edition (WEB.md)

> The design authority for the browser mirror. Visual reference: [mockup/gtmux-web.dc.html](mockup/gtmux-web.dc.html) (§01 to §04).
> The mockup includes proposals; sections below distinguish them from the current implementation.
> Implementation entry point: `internal/server/web/` (`index.html` / `app.js` / `style.css`). Everything underneath
> reuses the existing contracts (`/api/agents · /api/pane · /api/transcript · /api/diff · /api/icon`); the browser polls view data and fetches icons as needed. Authorized input uses `/api/send` and `/api/upload`; permissions and choices use `/api/share` and `/api/options`. The other panes, appearance and enrollment flows use `/api/panes`, `/api/theme` and `/api/enroll`. Owner profile sheets read `/api/participant-profile`; readable external sessions use `/api/session/transcript` with their exact session/home scope.

Uploads use multipart `file` with `POST /api/upload?id=<pane_id>`. Guest links require host input consent, current grants, and that link's input permission for the target pane, just like `/api/send`. Owner/device callers retain support for uploads without `id`.

## Positioning

A browser has a big screen and a real keyboard and mouse. The web mirror must not be a port of the phone's single column layout; it is a desktop workbench. An agent directory grouped by state and window on the left; drag a listed pane onto a board, arrange and resize freely, compose your own view the way you would in tmux. Typing is available only for panes the caller may control.

## Red line: layout stays local; input is authorized

Real tmux can split/kill/spawn; the mirror does not pretend to. "Arranging windows and panes" means the user's own viewing layout; **it never changes the real tmux tree**. Pane input is available only after the server confirms the caller's scope.

**When the shared page can type, a failure has to speak up.** A refused send must explain why and preserve the draft without overwriting a newer draft. `makeComposer` handles ordinary HTTP error bodies under the composer. The 403, 409 and network-failure paths restore the submitted text only while the box is still empty; a newer draft stays in place ([#1391](https://github.com/chenchaoyi/gtmux/pull/1391)). A network failure means delivery is unconfirmed, so the page must not claim the text was definitely not sent or resend it automatically.

Enter confirms an active input-method composition without sending it. Plain Enter inserts a newline; Ctrl/Cmd+Enter sends. Settings offers an explicit Return-to-send preference. The Send button uses an upward arrow; the separate terminal Return key sends Enter without text.

## 1. Top bar

gtmux logo · layout preset dropdown (Frontend trio…) · snap to grid toggle · Show waiting panes toggle ·
connection indicator (server name + status dot, never the word "live") · a Settings icon (System / Light / Dark, terminal font and size, and Return-to-send preference).

## 2. Left directory (session/window/pane tree)

- The workbench reads `/api/agents`, grouped needs-you→errored→working→idle→running, then by window. Multi-pane windows get a heading; the current headings do not expand/collapse. A search filter is at the top.
- Native sessions appear separately. A readable external conversation opens read-only Chat / Trace; other native entries say **Status only**. Neither kind can be dragged onto the board, added as a tile, selected in the pane palette or reached by previous/next pane navigation, which require a real tmux pane ID.
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
- Tile header: avatar + corner status badge · name · `Chat / Trace / Terminal / Diff` switch · ⤢ full screen · × close.
- A waiting tile gets a red border + a light pulse.
- Multiple panes = multiple concurrent `/api/pane?id` mounts; `diff` uses `/api/diff?id`; `chat` uses `/api/transcript?id`.
- Tiles use absolute positions: resizing one does not reflow the others. Clicking anywhere in its header except a button, without dragging, maximizes it within the board; `f` and `1–9` also maximize tiles. The workbench-only command palette adds or finds a tile, flashes it, and maximizes it in the board. Use Restore or Esc to return. This board maximization is distinct from the ⤢ focus view.

Tile headers keep mode labels on one line and wrap the control group when the tile is narrow; the input-capability chip remains visible.

## 4. Full-screen focus (single-pane close reading, mockup §02)

A tile’s ⤢ button or a narrow-screen radar row opens the single-pane focus view. For agents it offers Chat, Trace, then Terminal, previous/next pane, and terminal controls for font size, appearance, copying the selection or visible screen, and jumping to the latest output. Diff remains a workbench tile mode; focus has no diff tab or wrap toggle. Esc returns to the previous top-level view.

**Below 800px (a phone).** The focus bar groups Back, title, connection and Settings above one row of modes. Identity metadata stays under the title; previous/next pane buttons are omitted at this width. The title truncates before pushing controls offscreen. Read-only explanations remain visible where input would otherwise appear. The appearance panel opens under the bar.
This also applies to plain shell panes, whose Chat/Terminal tabs are hidden.

**Codex's pinned prompt.** Codex pins the prompt of the turn on screen to row 0, cut at the pane's width with "…". In the single-pane terminal view the full prompt from the conversation log takes a bar above the terminal (two lines at rest, a click opens it, Copy) and the cut row leaves the terminal. The rules are the phone's (MOBILE.md), run by a JavaScript copy in `app.js` against the same case file (`mobileapp/src/ui/codexPinnedCases.json`) and the same tmux-measured cell widths, to check agreement on those cases and width tables. While the row is on screen but unexplained, the view fetches the log at most every 4 s. Not in workbench tiles: there is no room for a second bar in a tile, and its row stays as captured.

## 5. Chat mode · wide-screen edition (mockup §03)

Same source as the mobile `ChatView` (`/api/transcript`: prompt → collapsed intermediate steps → agent reply), re-laid-out for wide screens:

- Turn directory on the left: lists every turn, `j`/`k` to jump, current turn highlighted (global navigation only the big screen has).
- Centered chat column (up to 900px including padding): user bubbles on the right with the human avatar; agent bubbles on the left with the official icon; icon-only Copy actions have localized hover hints and accessible names, work with a mouse, keyboard or touch, and briefly show a checkmark or failure icon after use.
- Waiting card: `/api/options` supplies parsed numbers and labels. A caller authorized to type can click a choice to send its digit via `/api/send`, without Enter; read-only callers see the choices without send handlers. Only parsed options become buttons: empty results or a failed options request clear the choices, never invent `1/2/3`. A typable chat view offers its composer below the conversation; a read-only view says to use the phone/Mac. Waiting can be an open question, not necessarily an approval.
- Intermediate steps can be expanded/collapsed. Free-text input is available in **Chat** and **Terminal**: the shared multiline composer uses Enter for a newline and Ctrl/Cmd+Enter to send, with staged attachments and a control-key strip. The chat surface follows browser appearance. Copy copies the message body; Quote is not offered.
- Refresh compares the complete returned turns, including segment text and tool details, in focus and tiles. An unchanged poll preserves the view; updates keep the existing step expansion and scroll position in focus.

## 6. Your avatar · the human in the agent era (mockup §03 appendix)

The current browser uses a fixed person-battery SVG on a cyan gradient disc for an unattributed human prompt. A turn with sender attribution uses the HQ wordmark or the sending agent’s avatar instead (`senderAvatarEl`). The color does not identify a person.

The mockup’s other variants (Ascension / Conductor / Decider / Captain / Resting / Rubber stamp / Dog-walk reversal / Hamster wheel), avatar photo upload, emoji and initials are **proposals, not implemented browser settings**. This does not restrict attachments in the composer. They do not establish that all three surfaces have an avatar picker.

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

The Web page sends through `POST /api/send`; it does not use the CLI’s raw `/api/attach` WebSocket. Once `/api/share` has resolved, authorized Chat and Terminal views show the composer without a redundant writable badge. Read-only views retain a capability label and a note instead of an input area. Parsed choices in focused Chat may also be clicked by authorized callers.

Owners can type into panes their owner credential controls. A guest needs the share link’s input scope (input ⊆ visible), the host’s “allow collaborators to type” switch, and grants that are still current (`GrantsStale` is false). The top bar distinguishes owner and guest. The server checks authorization on requests; after revocation, new requests with that credential are rejected. This HTTP statement is not a claim about the separate CLI attach stream’s scope or shutdown timing.

`GET /api/share` with `stale:true` keeps the guest connected but shows “Share access has expired. Ask the owner to grant access again.” in the scope strip and empty radar/directory. Successful roster polls refresh this capability, including recovery after re-granting. Ordinary empty guest views say there are no shared agents; only owners receive instructions to start an agent in tmux. A stale grant never becomes a token-revocation (`401`) message.

## Browser entry under a Direct path

A share URL may end at `/p<port>#code=...`, with no trailing slash. The HTML establishes a same-origin document base before loading any stylesheet or script. Static resources and API requests use the same Mac prefix; root and explicit `index.html` entries are supported too. Query strings and credential fragments never enter the asset base. Existing share links do not need replacement, and guest access remains governed by server scope checks.

### Shared mode semantics (2026-10-10)

Focus and workbench order Chat, Trace, then Terminal. Chat and Terminal retain the same relative order as the native App; native App Trace is a proposal, not a delivered mode.
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


### In-conversation interaction

Waiting shows the current prompt and safe numbered choices in context; answering folds the card, and resuming work removes it. There is no separate Interact mode or duplicate keyboard. Unrecognized prompts use the existing composer Keys and Terminal. App, iPad and Web share the read-only current-screen endpoint, preserving hook, scope and key-outcome boundaries. See [design and acceptance boundaries](session-interaction.md).

### Opening a pending question

A host-advertised View question action opens the focused session's Terminal after the shared input gate accepts its revision-bound action. It does not submit the composer draft, choose an answer or overwrite the saved Chat preference. Unknown, expired, denied and uncertain input remains blocked/recoverable through the existing key controls. Workbench tiles already expose question actions only in Terminal.

## Local workspace and transcript trace

The menu-bar footer opens an authenticated `http://127.0.0.1:8765` browser workspace using a one-time enrollment code. It reuses a running serve or starts an app-owned loopback child, without enabling a tunnel. Remote access changes in the app release that child before starting the managed service. Closing the app stops only its own child.

Focused Chat and board tiles share message, avatar, copy and tool-summary rendering. Supervisor sessions use the HQ role mark; stitched histories use each turn's source agent. Both Chat and Terminal use the scope-gated composer. Drafts stay per pane across polling and mode changes; text is also saved locally within the server and credential scope. Revocation clears drafts and recall data.

Trace indexes public prompts, assistant segments and recorded tool input, falling back to summaries for older records. It shows working directory and truncation when recorded, with search across all loaded records, type/history filters and a keyboard/touch overview. Pane and readable external-session routes keep their existing scope. Model request timing, tokens, private reasoning and tool output are not part of this contract. Wide views split list/details; compact panels drill into details and return to the same row. The list renders at most 200 records per page. See [trace behavior and native App proposal](transcript-trace.md).

## Browser appearance and identity

Appearance defaults to System and offers explicit Light and Dark choices, stored per browser origin. The page resolves the preference before styles load and follows live system changes when System is selected. Navigation, conversation, trace, controls and overlays use shared adaptive tokens. Terminal capture colors remain those of the server terminal.

The workspace brand uses a 26px mark, 22px name and a 72px header with deliberate gutters. Session search has its own full-width row below the sidebar heading. The bundled SVG favicon identifies browser tabs, and tab titles include the current conversation and mode or the workspace/access context.

Trace details have a Close button and a toolbar toggle. Closing gives the list full width and restores focus to the selected row. Refresh/filter changes preserve the closed choice, and selecting a record reopens details. Stable keys preserve identified records across history additions; duplicate clockless records remain distinct within loaded order. Follow latest pauses on inspection or manual scrolling. Selection/focus holds the displayed detail revision until released. Trace preserves reading space; Chat and Terminal retain their composers.

## Browser interaction and content rules

Use the bundled Lucide 0.468.0 paths (ISC license in `internal/server/web/vendor/lucide.LICENSE`) for interface actions. Action icons use a 24-unit drawing grid, a 20px displayed size and a consistent 1.8-unit stroke. Composer Send, keyboard and history icons use 24px. Informational capability icons use 16px. Do not substitute font characters for navigation, expand, close, copy or send. Agent artwork and semantic status marks retain their own identity.

Separate icon size from target size: desktop controls have a 36×36 CSS-pixel target; coarse-pointer controls have a 44×44 target. Compact tiles keep these same controls. Keyboard keys use readable Tab, Esc and Ctrl C labels, with arrows for direction. These project defaults exceed the [WCAG 2.2 minimum target baseline](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html) and use the [enhanced 44px baseline](https://www.w3.org/WAI/WCAG22/Understanding/target-size-enhanced.html) for touch. Trace overview marks are decorative; a 44px native range control and larger event rows provide navigation.

Controls have localized names, hover feedback and a visible keyboard focus ring. Essential action glyphs aim for at least [3:1 contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html) against their background. This is a design rule, not a claim of complete WCAG conformance. Layout and appearance disclosures expose their expanded state, close with Escape and return focus. Session rows support keyboard opening, and sidebar refresh retains a focused session. The command palette keeps keyboard focus until dismissed.

Message backgrounds contain the body only. Copy sits in a separate transparent footer below the message, aligned right for user messages and left for replies. A prompt's recorded timestamp and sender attribution share that footer; missing or invalid timestamps are omitted. Clipboard actions report the actual result, including fallback failure, with an icon and an accessible status message. Quote is not offered.

Chinese interface terminology: **工作台** for the browser product, **工作区** for the board, **面板** for a tile, **会话** for an agent conversation and **窗格** for a tmux pane. Keep protocol IDs and commands literal. Use concise actions (保存当前布局、恢复布局), factual empty states (暂无已保存的布局), and specific recovery instructions. English uses sentence case. Do not edit agent transcripts as interface copy. Appearance settings explicitly label terminal font and text size because those controls affect terminal rendering.

## Working conversations

Focused Chat and workspace Chat panels show a Live disclosure above the composer while the agent is working. It starts collapsed, remembers the reader's choice per pane through refresh/mode changes and disappears when the agent stops working. Expanded Live reads the last 14 current-screen rows from `/api/pane`, matching the app's data source; it never substitutes an old transcript reply. Terminal escape sequences are removed and content is rendered as text. Runtime uses the state-start timestamp and HTTP server clock, and is omitted when either is unavailable.

Focused Chat refreshes the agent roster alongside transcript polling so its status does not freeze at entry. Only expanded, visible, working previews request captures. Capture errors identify unavailable or stale output and retry. Live preserves a scrolled-up preview, drafts and transcript reading position. Requests are retired on view/mode changes and checked against pane, working turn and credential; revocation clears cached Live text.

## Composer and speaker profiles

Chat, Terminal and workspace tiles share one composer. Its control strip offers Tab, arrows, Return, Backspace, Ctrl-C and Esc; those keys send terminal input separately from the text draft. Desktop composition starts open. On touch screens the keyboard button opens or collapses the field; controls use 44px targets, safe-area insets and the browser visual viewport. Expanded writing uses the same draft and keeps Send reachable above the keyboard.

The attachment sheet offers photos, files, clipboard images and a camera capture option on touch devices. Pasted and dropped files stay staged until Send. Images can be marked with a pen, arrow, box or opaque redaction, with undo and clear; export flattens marks at the original dimensions. A smaller export is offered only after export failure. Upload progress is visible; failed uploads retain the text and files for retry. Files stay in memory and must be selected again after a page reload.

Text drafts, input history and snippets are saved locally within the server and credential scope. History and snippets insert text for review rather than sending it. Changing identity isolates drafts; revocation clears saved composer data. After sending, a pending message bridges the delay until the transcript records it; recorded messages replace it without duplication. An unchanged retry reuses the send identifier to avoid duplicate delivery. Network failure remains an unconfirmed delivery state.

User, sender, Agent and HQ avatars open an identity sheet, using iPhone/iPad's hierarchy of avatar, identity and grouped facts. Owner agent sheets read the existing participant-profile endpoint and show recorded model, version, provider and permission facts with source/time disclosure. Message avatars use their recorded session and prompt time (`at`), including older sessions of the same provider; they never inherit the current session's status, project or navigation. Missing message identity and guest connections do not fall back to a current-profile request. User messages do not imply a personal profile or device that the transcript never recorded. Closing or changing credentials retires pending reads; sheets close with Escape or the Close icon and return focus. On phones they appear from the bottom. HQ-specific execution-chain and extension views remain native App capabilities.
