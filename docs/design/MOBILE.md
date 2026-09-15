# gtmux mobile — design supplement (app icon · agent icons · visual rules)

> This file is the **design-layer supplement** for the mobile app, meant to be read alongside the engineering blueprints:
> - `mobileapp/SPEC.md` — the build blueprint (stack, screens, dependencies).
> - `api/contract.md` — the HTTP/SSE `v0` contract.
> - `mobileapp/src/ui/theme.ts` · `StatusBadge.tsx` — tokens and the status badge (authoritative).
> - `docs/design/DESIGN.md` §0–§3 — the status language (shared by all five surfaces, see `SURFACES.md`).
>
> Visual reference: `docs/design/mockup/gtmux-mobile.dc.html` (interactive; four screens + push + icons).

The mobile app is gtmux's **phone and iPad form** (one app, two shells: compact / regular, §5): the desktop's remote companion. A phone cannot run tmux, so the app is a pure consumer of
`gtmux serve`, reached over VPN/Tailscale, and a **read-only MVP** (monitoring + focus + push).
Its status language is identical to the menu bar's: **colour + shape + glyph**, where colour encodes status only and never an agent's identity.

---

## 1. App icon (gtmux mobile)

The brand motif is the **pane grid**: a 2×2 grid with the top-right cell lit **cyan `#06B6D4`**, meaning "the pane that is focused / waiting on you". Dark ground, restrained, legible at small sizes. **The app icon shows no status count** (that is the menu-bar status item's job).

### Construction

- Full-bleed square canvas; rounding is left to the iOS squircle mask (preview the design at ~22.5% corner radius).
- Background: `linear-gradient(160deg, #262B36 0%, #0E1016 100%)`, with a 1.5px inner highlight along the top
  `inset 0 1.5px 0 rgba(255,255,255,0.08)`.
- Grid: centred, about 58% of the icon's width; 3 neutral cells `rgba(255,255,255,0.22)` + the top-right cell in cyan
  `#06B6D4` (with a soft outer glow `0 4px 14px rgba(6,182,212,0.5)`); the bottom cell spans both columns.
- Grid layout (matches the brand logo):

  ```
  ┌──────┬──────┐
  │ 中性 │ 青色 │   ← 右上点亮
  ├──────┴──────┤
  │    中性     │   ← 底排跨两列
  └─────────────┘
  ```

  (neutral · cyan, top-right lit; the bottom row spans both columns.)

### iOS 18 variants (required deliverables)

| Variant | Background | Grid |
|---|---|---|
| Default | `#262B36→#0E1016` gradient | neutral white 22% + cyan |
| Dark | `#000000` | neutral white 16% + cyan |
| Tinted | `#1A1A1D` | monochrome: neutral white 30% + bright white 85% (the system tints it) |
| Light | `#EEF0F3→#DADDE2` gradient | neutral black 16% + cyan |

### Export

The full iOS size set 20–1024 (@2x/@3x): notification 40, settings 29, spotlight 20, home 60,
App Store 1024. Redraw each size from vector (the grid is plain rectangles + rounded corners) rather than downscaling, so the grid does not smear at small sizes.

---

## 2. Agent icons (the row avatar)

Every radar row starts with an **agent avatar** that tells you *which tool* it is (Claude Code / Codex / Gemini …).

### Rules

1. **On a real device, show each tool's official icon**, loaded at runtime from `Agent.icon` (`gtmux serve`'s `agentJSON`
   already carries the field: an `.app` path or an image).
2. **Official logos are third-party trademarks — never redrawn in the repo, never bundled** (DESIGN §6).
   The iOS side resolves `Agent.icon` into a loadable source; when it cannot, it falls back to a neutral letter mark.
3. **The fallback is a neutral letter mark** (IP-safe, for telling agents apart, not a logo):

   | agent | mark | agent | mark |
   |---|---|---|---|
   | Claude Code | `CC` | Cursor | `Cu` |
   | Codex | `Cx` | Crush | `Cr` |
   | Gemini | `G` | Amp | `Am` |
   | Copilot | `Co` | Cline | `Cl` |
   | opencode | `oc` | others | first 2 characters of the name |

4. **Colour still belongs to the status badge alone**: the avatar container stays neutral (`surface` ground); agents are never colour-coded.
5. **One failed fetch is not a verdict.** Every row of the same agent requests the **same URI**, so "this row shows a letter mark while its neighbour shows the icon" never means the agent has no icon — it means one request did not land (measured 2026-08-29: on 4G, 14 Claude rows with identical `icon` hints from the Mac, one of them a letter mark). A failed fetch gets **bounded retries**
   (800ms / 2.5s / 6s); while waiting, show the letter mark (never an empty square); only after all three fail is it a conclusion. A changed `Agent.icon` is a new question and inherits no earlier verdict.

### Avatar container (app-icon style)

- **34pt**, a **rounded square with radius 9** (not a circle — the rounded square signals "an app icon goes here"),
  `overflow:hidden` so square official icons sit naturally; a 16pt status badge overlaps the bottom-right corner.
- `AgentRow.tsx`:

  ```tsx
  {agent.icon
    ? <Image source={ {uri: resolveIcon(agent.icon)} } style={appIcon} />
    : <Text style={mono}>{agentMark(agent.agent)}</Text>}
  ```

> Note: this changes the existing `AgentRow` avatar from a circle to a rounded square and wires in `Agent.icon`. The rest of the row structure
> (primary bold · secondary grey · task · time · ›) is unchanged.

---

## 3. Radar interaction (the list)

### Collapsible sections (must be discoverable)

Every status section header is a **tappable collapse bar** — discoverability is a hard requirement, not a small chevron on its own:

- Left: the section name (red for waiting, neutral otherwise) + a **count bubble** (`surface` ground, stroked, radius 9).
- Middle: a `0.5px` rule stretches the header across the full width.
- Right: **explicit text `Hide / Show`** + **an arrow in a circle** (expanded ▼ points down / collapsed ▶ points right, `rotate(-90deg)`).
- The whole bar has a press highlight (`style-hover` → `rowSel`).
- The count bubble stays when collapsed, so a folded section still tells you how many it holds. The state can persist (kept on next launch).

### Separation between sections

Between adjacent sections (except before the first) sits a **divider slot**: a `9px` gap filled with the page background + a `3px` heavy rule along its top
(dark `rgba(255,255,255,0.16)` / light `rgba(0,0,0,0.16)`). "Needs you / working / idle" then read as separate blocks at a glance, not one continuous column.

### Everything else

- A "waiting only" filter; pull to refresh; initial `GET /api/agents` + SSE-driven refetches.
- Top bar, right side: a connection dot (live / reconnecting / offline) + a gear into Settings.
- A waiting row: faint red ground + the red square·double-bar badge + a single pulse.
- **The two top-bar buttons each own a 40pt square and never overlap** (2026-08-16). They used to be "14pt between the icons + a 10pt hitSlop on every side of each" — **the two hit areas overlapped by 6pt in the middle**, and the overlap belongs to whichever renders later, so aiming at the right half of "all panes" opened Settings. **Visible spacing is not tappable spacing**: adjacent buttons are judged by their hit areas, not their icons.
- **The list for picking a pane for a link has the same shape as the pane browser**: grouped by **session**, every row led by **`%N`**, then the task name (2026-08-16). It was a flat list of task names, so two panes running the same project in two different sessions **looked identical** once truncated, and the user had nothing to tell which one they were authorising. An authorisation screen least of all can run on "probably this one" — the id is the anchor, the name only annotates it, and that matters more here than anywhere.
- **The list must say where it ends, and draw it rather than write it** (added 2026-09-05, revised 09-06).
  It used to stop at the last row with empty space below, which reads as "still loading". The first fix was a line of small text,
  `end of list` / `到底了` — **a lowercase English fragment lying under a clean list**: it has to pick a capitalisation and it has to be translated, and neither decision has a good answer. It is now **a short centred rule**: no case, no translation, and it reads as "that's all" in every language. **The rule must be short**; a full-width rule is just another row divider.
  **Only when a section is collapsed does it add `showing 5 / 16`** — then "that's all" is no longer true, and that fact cannot be drawn. What is added is a **count, not a sentence**, so there is still no capitalisation to get right.
  It does not report the total otherwise: this list's total is not the fleet's total (the chief of staff sits on the floating disc), and a footer saying 16 under a top bar saying 17 sends the reader hunting for the one that got away.
- **Two variable-width things on one line need separate budgets** (2026-09-05). The secondary line is "text + branch pill": the text was `flexShrink: 0` and the pill compressible, so a long rate-limit error on that line squeezed the pill to zero width — **a zero-width pill does not disappear, it is its own padding and stroke**, an empty little white capsule on screen.
  Now the text yields first and the pill keeps its natural width, capped at half the line; whichever is too long ellipsises within its own budget.
  Also, **when a row reports a failure, that sentence owns the secondary line** (no branch attached): trading "resets Aug 28 at 11pm" for a branch name swaps out the wrong half; the branch is still there on the long-press card and in Detail.
- **A status word appears only when nothing else is already saying it**: offline and "access denied" each already have a full-width banner directly above, so writing `⊙ offline` in the top bar too says the same thing twice while squeezing the Mac's name into an ellipsis and the buttons together. Only "reconnecting" has no banner, so it keeps its text. The status dot is always there.

### Long-pressing a row = what the row could not fit + the things you can do in one step (2026-08-29)

The row clamps the task to one line for density; **the long-press exists to give back what was clamped**. It used to open a system Alert
showing the agent name + the task — **both already on the row being pressed**, so a deliberate 350ms gesture bought a sentence you had already read.

A bottom card, four blocks, each shown only when it has something to say:

1. **The header is the anchor, one line.** `session · %N`, with status + duration + project·branch on the line below.
   **The title does not name the agent** (2026-09-03): the avatar already says which tool, the row's second line already says where, and spending the largest type on the page on something the reader knew before pressing is this card's most expensive waste.
   Identity no longer gets its own colour block either — that read like a system diagnostic, not something you can act on.
2. **What it is doing.** The full task (untruncated), the full error text, background work. This is why the card exists, so it follows the header directly, ahead of every action.
3. **What can be finished from this row** (reordered 2026-09-03). The first version was ordered by "easy to implement": open, copy the jump command, jump on the Mac, view changes — three of the four are worthless to a reader **holding a phone, away from the Mac**:
   "open" is what tapping the row already does, the copied command has nowhere to be pasted, and a jump only makes sense with a person at the Mac.
   The phone's use here is **unblocking**, so the order follows that:
   - **Answer it** (only when `waiting`): fetch `/api/options`, lay out the agent's own 1/2/3 text as buttons,
     **ahead of every action**. Digits are sent as **keystrokes**, never pasted (a numbered menu recognises keys; pasting a "1" selects nothing).
   - **Jump on the Mac**: **second, never last** (corrected 2026-09-03). The first version pushed it to the bottom on the grounds that a jump is useless away from the Mac, but the real usage is **the phone as the Mac terminal's remote control** — one of gtmux's founding capabilities (`terminal-jump` is a capability in its own right). It is also the only action whose meaning is the same in every state. In sixth place, below the task text, it fell straight off the phone's screen and read as "this feature is gone".
   - **Let it continue / Interrupt it / Ask the chief of staff**: the things most often said to an agent.
   - **View changes** (when there is a repo): the read-only one goes last.
4. **The actions are a group of icon-led list rows** (2026-09-03) in one rounded container with hairline dividers, not four separately bordered cards (four competing buttons are not a menu).
   - **Icons are drawn as vectors** (`react-native-svg`), never faked with text characters: `▶` and `■` differ in ink, optical weight and baseline in the body font, and a column of them looks like a ransom note. Shared 24 viewBox / 1.7 stroke.
   - **Every action must say exactly what it sends.** "Continue" is a label, not a contract — the subtitle reads
     `sends "continue" and Enter`, `sends Esc, stops the current turn`, `switches the Mac's terminal to %N`.
     The reader's first question is "what does this button actually do", and a label cannot answer it.
5. **Feel**: spring entrance (not a linear slide) + press feedback. **The entrance may run only once** — the radar re-renders every 1.5 seconds, and putting the `agent` object or an inline callback in the effect's dependencies makes the card re-spring while you are looking at it (measured: one re-entry per render). The dependency is `pane_id` only; everything else goes through a ref.
   **Haptics are not there yet** — they need a native module; listed separately.
6. **Nothing else.** **No irreversible operations** — the long-press only **opens** the card; doing anything takes a second deliberate tap inside it, and the heaviest of those merely interrupts a turn (the next "continue" brings it back). No kill, no reap, no clearing a session. Nor anything Detail already has (the input box, font size) — those are one tap away.

**Four kinds of row each tell their own truth** (the easiest place in this block to get wrong): the radar holds agents inside tmux, sessions sensed outside tmux, and ordinary panes the user pinned. They used to pop up identically — for the latter two that was not "abbreviated", it was **wrong**:

- **A session outside tmux**: state plainly "sense only"; jump/copy are **greyed with a reason**. Simply hiding them makes people think they missed something; showing them dead is worse.
- **A watched ordinary pane**: give `%N` and its command, **no agent status** — it has none, and writing "idle" is inventing a fact.
- **An errored session**: the error text is the most important thing on the row, and it is precisely the one that got truncated.

### The "all panes" browser (`PaneBrowserScreen`, entered from the ▤ in the radar header)

The radar stays agent-first; the full set of tmux panes lives on this separate full screen (same red line as DESIGN §16: **never flattened into the radar**).
Session cards fold (the header carries a status rollup and still speaks when folded), the search field is always present, tapping a row opens Detail.

**The identity rules are shared verbatim with the menu bar and the browser (tmux-id-surface, 2026-08-14) — the id is the anchor, the name only annotates:**

- The row begins with **`%N`** (the pane's tmux id), not the shifting `w.p` coordinates. **Tapping it copies `gtmux focus %N`**
  and **echoes `✓ copied` in place** (a phone has no other feedback; a successful copy and a dead tap look the same). That tap belongs **only** to the id: the whole row still means "open this pane" (a nested touchable swallows the parent's onPress, which is exactly the behaviour wanted).
- **Window bands** (`@id name`) **are inserted for every session**, including single-window ones (reversed 2026-08-14: the old "save a row" was counting rows, not reading the tree; a single-window session put its panes directly under the session header, and the indentation meant something different from everywhere else). **The session header always lists all its window ids** (more than 5 fold into `+N`) — a folded session must still say what it contains.
- **The secondary line does not name the agent** (the icon already does); only genuinely supplementary things like the directory remain.
- **Indentation must actually indent**: pane rows sit **inside** a window band, so their left margin must be **larger** than the band's (26 vs 22).
  It was once the other way round — rows at 14, bands at 22, **the child further left than the parent** — so on the phone it read as a flat list while the menu bar was a tree, and the two screens did not look like one product. The band's ground is a **faint value of the text colour** (neutral grey at 10%), not `surface`: in the light theme `surface` is pure white and the page is `#F2F2F7`, so the band would be brighter than the content it groups, the opposite of a ground tint. A 7pt gap sits above the band and **outside** its ground — it is separating two windows.
- **Search understands ids**: `%23` / `@17` / bare digits all match.
- **An agent row's first line says "what it is doing"** (the task the radar already derived), not the agent's name — when six panes are all called "Claude Code", the name is not identity; the avatar already carries that.
- **Until the first read of the list lands, the page says it is reading** (2026-09-15: 「点击 all panes 后需要增加 loading screen」). It used to open blank under the header until `/api/panes` came back, and a blank list is indistinguishable from "no panes". Now the list area carries `ui/LoadingMark` with "Reading panes on <machine>" beneath it, and the header's count line reads "reading…" instead of "0 panes · 0 sessions" (a zero on a machine with twenty panes is a false statement, not a placeholder). The mark leaves the moment the rows arrive and never shows for an empty result; the empty statement is for that.

---

## 4. Detail interaction (pane view + input)

### Long-pressing a row: feedback has three layers (2026-09-10)

The card that a long-press on a radar row opens (`RowSheet`) used to give **no feedback at all**: the row faded to 0.6, and 350ms later a menu sprang up from off-screen.
All three layers were missing, and missing any one of them still reads as "no feedback":

- **A medium impact at the moment of recognition.** This is the shared language of long-press menus on iOS, and the app had not a single vibration before.
  A thirty-line native module was written for it (`ios/GtmuxMobile/Haptics.swift`), no third-party dependency — this repo already writes its own native modules. **It exports only the two actions the product actually uses** (prime on finger-down, tap on recognition); haptics are easy to overdo.
- **During the 350ms of holding, the row must move.** The whole row eases from 1.0 to 0.965 (`ui/pressFeel`), so the finger can see it "charging";
  letting go midway springs it back, which is itself the answer "didn't take". It runs on the native driver, because the radar re-renders every 1.5 seconds.
- **The menu grows out of the row**, rather than flying in from off-screen: the rise distance shrank from 420 to 280.

### A sheet's container cannot be a Touchable (2026-09-10)

The long-press card used to be wrapped in two Touchables: the outer one for "tap outside to close", the inner one merely **swallowing taps** so the close did not fire.
**A Touchable is an accessibility element by default, and iOS collapses an element's whole subtree into it** — so VoiceOver could reach only "this card", not one action inside it, not the body text either. It took a real press through Appium to notice: a driven long-press could not find a single action row.

- The inner one exists only to swallow taps; it became a `View` + `onStartShouldSetResponder`, which is not an element.
- The outer one must be tappable, so it stays a Touchable but with `accessible={false}`.
- **Do not use a container's testID as the proof that "the sheet is open"**: neither container is an accessibility element now. Ask instead
  "is a given action present" — which is exactly the thing that actually needs to be true.

### Grouping the long-press menu: the groups were already in the data

Every action carries a group, `answer / go / drive / look`, and the view flattened all six into one block — so "Interrupt it"
(send Esc, stop the running turn) looked identical to "View changes". They are now drawn as blocks by the groups that already exist.

- **The order did not move at all.** "Jump on the Mac" in second place was settled on 2026-09-03 (the phone is the Mac terminal's remote control; buried under five rows it reads as deleted). Grouping makes the difference visible; it does not re-queue.
- **"Interrupt it" uses amber** (`ERRORED_COLOR`, with the darker `#B45309` for text so it reads as a label on white rather than a warning badge).
  **Not red**: in this product red means only the "waiting on you" state, and using it as a button colour would collide with the status language.
- **The header says one sentence, not two facts.** It was "waiting · 4 min", a status word and a duration joined by a middle dot;
  now it is "waiting on you for 4 min" / "running for 12 min" / "finished 3 min ago" / "errored, 10 min ago".
- **"Answer it" was renamed "Write your own"**: the numbered options already form their own block above, and what this row really does is open the session so you can type.

### Collapsing the top chrome: chrome is an overlay, not part of the layout (redone 2026-09-10)

The header + the neighbour bar + the **controls row** (the segmented control and the tool keys on one line, see below) collapse together under **one driver**: fold away while reading history to give the height back to content, return when you are back at the live tail. That is the design. **The mechanism took three versions to get right**, and every defect in those three versions came from the same decision — collapsing by *animating the chrome's height*, so the scroll viewport changed with it:

- The viewport is precisely the measure of "am I at the live tail"; collapsing changed the answer, and the answer asked for the opposite action — measured on a device: three reversals, 117pt of travel before it settled (「回到底部的时候会跳来跳去」, "it jumps around when returning to the bottom", 2026-09-05).
- When the viewport grows at its **top edge**, the content shifts up by the same amount: 115pt in 200ms that nobody asked for (「屏幕会向上弹跳一小段」, "the screen bounces up a little", 2026-09-09).
- Keeping the content still during the collapse means writing `contentOffset` every animation frame, and those writes cannot win under a finger:
  the pan gesture's next frame overwrites them with its own displacement, so scrolling appears **stuck at the fold point** (「到了折叠的地方就会停住」, "it stops where the fold is", 2026-09-10);
  changing to "collapse after the gesture ends" trades that for **a late collapse** (「遮掩、收回延迟感比较大」, "the hide/retract feels laggy", same day).
  The same writes also cut short a running `scrollToEnd` animation — that is the "tap the back-to-bottom arrow, the top flashes and retracts, and you never reach the bottom".

**None of these were misjudgements, so they cannot be fixed in the judging rules.** The chrome now **does not participate in layout**: it floats above the scroll view, slides out with `translateY` when collapsed, and the scroll view's frame never changes; the content carries a **constant** top inset
(`topPad` = the chrome's height), so the oldest line can still slide out from under the chrome.

With the geometry constant there is nothing to compensate, nothing to fight and no loop — `gap` means the same thing before and after a collapse.
That deleted three things: the layer driving the offset, the gate waiting for gesture end, and the "threshold floats with chrome height" that existed only to outrun the loop. **The fold now happens 72pt from the tail instead of about 250pt**, the visible result of deleting the third.
What remains is hysteresis, still worth keeping: it stops a scroll parked near the threshold from flickering.

- Expand: `gap ≤ 40`
- Collapse: `gap ≥ 72`
- In between: hold
- Anything measured mid-animation does not count (every frame of those 200ms describes a layout on its way somewhere else); ask again with the latest reading when the animation ends —
  there will be no second scroll event once the finger is already up.

The collapse runs on `transform` + `opacity`, so it lives on the UI thread (`useNativeDriver`): however busy a live pane keeps JS, it cannot slow it down.

Three companions:

- **Child views report a "distance", not a boolean.** The host compares it against the thresholds.
- **Following the live tail is the user's intent, and only the user can withdraw it** (`stick`, in both layers).
- **The "back to bottom" arrow may not announce a position it has not reached.** It used to report `gap=0` the moment it was pressed; the host believed it, expanded the chrome, and the expansion's compensating write cut short the very scroll animation the press had started. Whether you have arrived is for the scroll's own frames to say.
- **With both layers mounted, only the layer you are looking at may drive the chrome.** Detail keeps both the chat and terminal layers in the tree for instant switching, so **both were reporting**: measured with the terminal 1300pt deep in history, the log alternated
  `gap=258` (terminal) and `gap=0` (the chat parked at its own tail behind it), and the chrome expanded on the chat's reading.
  One driver, one source.

### Only three bands at the top (2026-09-09)

The top of Detail used to be **four bands**: the title, the neighbour-pane bar, a full-width `Chat | Terminal`, and a tool row
`● server · Diff · A− · A+ · ⛶`. **155pt in all, 21% of the usable screen height**, each with its own divider. The user said of the terminal page that "the top takes up too much space".

**Two control rows were one row of controls wearing two dividers.** The segmented control now moves into the left of the tool row (`segInline`,
sized to content, no longer full width — two targets of about 80pt, enough), the tool keys stay on the right. 40pt and one rule saved.

**The connection indicator merges into the title's subline**: a status dot leads `agent · status · pane`. This is a **deliberate narrowing** of D9
("server name + status dot"): while the link is healthy only the dot remains, and the subline's width goes to the words the reader actually came for;
**when it is not healthy, the machine name and the status word both appear** — exactly when "which Mac dropped" matters most.
The narrowing is written down here rather than left as a silent deviation.

**The collapse is now four blocks to three, but the rule is unchanged, and the tool row is included.** It used to be the one band that did not fold, which made the sentence above — "one gesture, the whole top chrome folds together" — untrue on screen. The point is that **whatever folds must count towards
`chromeH`**: the thresholds derive from "the height being toggled", and a band that folds without being counted makes the oscillation `liveEdge` works to remove possible again. `detailChrome.test.ts` reads the source directly to guard this —
it guards structure, which a render test cannot see.

Result: **115pt / 15%**, about 5 more terminal rows.

### "Still waiting on your call" moved to the top of the board (2026-09-09)

It used to be a `###` under `## ① 现状` (current state), **visible only after expanding the section above** — and it is the only part of the whole board that belongs to **the commander** rather than to HQ. Both ends now lift it to the very top, in waiting red.

**The cost, stated plainly: this requires gtmux to own that heading.** Every other heading on the board is a name HQ chose, and a surface can only lift a section it recognises. So the heading is written into `boardSeed` (playbook v36), matched in both the Chinese and English spellings (a board keeps the language it was seeded in); **a board matching neither shows no band** — an honest degradation, not a guess.

**Empty is the normal state, and empty is not drawn.** The charter says so in as many words: a band that is always there gets ignored, and this section is the only thing on the board entitled to his attention.

The heading now lives in source in three languages (the Go seed, Swift, TS), exactly the shape that drifts —
`internal/hq`'s tests treat the Go seed as the reference and check the other two against it; change one without the rest and the build is red.

### A decision on the board is given from the item (2026-09-14)

The commander's section is a numbered list of decisions only he can make, and HQ writes its recommendation into most of them — and the sheet was read-only, so answering item 3 meant closing it and typing "about item 3…" into the composer from memory (「没有直接处理告知 hq 的入口」). Now each item is a row (`boardSections.askItems` reads HQ's numbering and bold group headings; the group stays as a label) with **Tell HQ ›** at the right. Tapping asks the two things a commander says to a chief of staff: **Do as you suggest** — offered only when the item carries a recommendation — sends 「态势板「还等你定的」第 3 条（…）：按你的建议办。」 at once; **Let me say…** closes the sheet and puts that quote in the composer with the cursor after it. The quote names the item by HQ's number and first line, so HQ knows what was decided without re-reading. The board's text is untouched; the reply is the ordinary send. The Mac reader stays read-only by DESIGN §12's rule — a reply is driving, and driving lives here.

### The board must say how fresh it is (2026-09-09)

The phone's situation board said "updated 3 hours ago" under its title; the Mac said nothing — yet `updatedAt` had long been in the Swift model, just never drawn. **A stale board read as the current state is a failure mode with real cost**:
it is HQ's understanding of the fleet, and the whole reason to open it is that the understanding can be trusted.

The banded copy is a pure function (`boardAgeText`), tested **at every band's boundary** rather than some convenient point in the middle —
a one-second-off boundary is exactly the kind of thing that looks fine in a screenshot. The Chinese is not a word-for-word rendering of the English: Chinese puts the time first.

### The menu bar's knowledge base: list and body side by side (2026-09-09)

The window is 640pt wide, yet the content was one column with 12pt padding — reading an entry meant leaving the list and coming back, a screen change for "what does this one say". Now it is **a 292pt list on the left, the body on the right**, and that round trip is gone; the back control went with it (the list is beside you, not behind you), but the body keeps a topic-name line at its top — it says where the entry lives, which the list only implies.

Window minimum width 520 → 700, default 640 → 820: that 520 floor was for a single column.

**The phone keeps drilling in.** It has no width to split; its answer to the same question is the 1.0.6 one — backing out returns you to the same list at the same position.

**Search follows the same rules as the phone** (title/id/topic, case-insensitive, whitespace-tokenised with every token required), pinned by tests on both ends. One query must behave the same on both screens, or one knowledge base will feel like two.

### The knowledge base must be findable (2026-09-09)

**Search.** 396 entries, 7 topics, and until now the only way in was "you know which topic it is in" — knowledge about the knowledge base, not about your machine. Matching is by title, **id** and topic, case-insensitive; whitespace tokenises, and
**every token must match** (two words narrow, they do not widen). Ids are searchable because HQ refers to entries by id when it dispatches work; that is what a person will actually paste.

**While searching, the index gets out of the way** rather than both showing — the index *is* the answer to "browse", and two answers on one screen is confusion. Clearing the input brings the index back. The search box appears only at the index level: inside a topic or an entry the question has already been narrowed.

**The four lines of explanation under "waiting for you to take" collapse into "What is this ⌄".** They describe the mechanism of promotion itself, worth reading once; left standing they are just height. It opens collapsed every time (by a reset on open, not an initial value — so last time's open state does not carry over).

**Name by action, not by the CLI's verb.** "Retire this entry…" describes a mechanism; what you are really saying is that this lesson no longer holds — which is precisely what the dialog is about to ask you. Now the button and the question say the same thing.

### The HQ page: it is a report, not a dashboard (2026-09-09)

**The three doors are there from the first frame (2026-09-15: 「需要等一阵才分别出现…比较唐突」).** Each door's first fetch used to decide whether its tile existed, so the row assembled itself one tile at a time in front of the reader. Now a door still on its first fetch is drawn in place with `ui/LoadingMark` (the pane-grid brand mark in the faint ink, breathing slowly) where the value will be; the value replaces it when it lands, and a door with nothing to show leaves no tile. The breath stops when the mark unmounts, so the zero-animation-at-rest rule holds. It is the one loading placeholder in the app; use it wherever a value is on its way.

**HQ's three documents are always present, not hidden in a fold.** Board / knowledge base / usage used to be GridRows inside a fold, and the fold `briefOpen` defaulted to `false` — open the HQ page and **none of the paths** to those three showed, with no other entrance on the phone. Now they are a permanent row of three small cards, each carrying its own live value (how fresh the board is, how many entries the knowledge base owes you, how much of the weekly quota). **The card that owes you something uses waiting red**, judged by the model's own
`owed` row — never by sniffing for digits in a string ("352 entries" is size, not debt; that would cry wolf in the most ordinary state).

This **overturns** the old "documents and figures are rows of one grid, sharing a key column": a shared key column made them look like more readings, and a destination you have to lift a lid to discover is not a destination.

**The verdict sentence needs the weight of a conclusion.** 14pt → 17pt. It is the product of the whole page, and it weighed the same as the sensor readings below it.

**The three tabs are three different things** (a queue / a log / a conversation), and only the queue can be urgent. They became pills, and the selected one carries its own status colour: red when someone is waiting on you, neutral otherwise.

**The decision card's buttons are 44pt** (`paddingVertical: 8` with 13pt text came to ≈ 30pt, below the touch minimum), and the primary action "Open session" uses brand cyan — it is the thing you are actually there to do.

**The quiet state must tell the truth.** "Nothing needs your call right now." plus a screen of blank space is this section saying nothing in its **most common** state. Now it says nobody is waiting on you, then lists the threads running right now (they do not need you; they show where the time is going, and a tap goes in), with an "Ask HQ what's happening" underneath.

### The HQ page's top is an overlay too (2026-09-12)

(Compact shell. In the regular shell the header is static and does not fold; the sections sit in the right-hand inspector, see §5.)
The HQ page's header (the verdict sentence, fleet counts, the three doors) and its section-tab row use the same mechanism as Detail's top chrome
(see "chrome is an overlay, not part of the layout" above): **one driver, both bands fold together**, sliding out with `translateY`,
the scroll view's frame below unchanged, each of the three sections carrying a constant top inset (`chromeH` = header height + tab-row height).

It was changed a month after Detail. For that month the HQ page still folded by "animating height", so on this page the user hit the loop Detail had already cured: a light upward swipe folded the header, the viewport grew by the header's height, the conversation's distance from its tail shrank by the same amount, the judgement asked for the header back, the viewport shrank again — fold, expand, fold, settling only after a swipe longer than the header in one go
(「轻轻滑动一下到上部折叠，会出现反复折叠展开来回换的现象，只有上滑比较大一截的时候才会稳定」, "a light swipe up to fold the top makes it fold and unfold back and forth; it only settles after a fairly big swipe"). An ordinary session page has no such problem, precisely because its chrome no longer participates in layout.

**The two top-anchored sections (Your call / HQ's work) do not fold; their chrome scrolls away with the content.** Their content reads downward from under the chrome, and a fold at 72pt would leave a blank strip of chrome-height-minus-72 above the first row (about 140pt measured in the simulator).
So in those two sections the chrome's `translateY` is bound directly to the scroll offset, clamped within its own height, tracking the finger on the UI thread, and it comes back when you scroll to the top; with no judgement there is nothing to flicker. Only the conversation area (pinned to the tail, reading history upward) uses the 72pt fold rule.

Two things on the way: the "HQ / fleet" toggle row in the HQ's-work section moved from above the scroll view into it (fixed above, it would either be covered by the overlay or leave a blank strip once the overlay scrolled away); and `ChatView` now pins the tail on content-size changes too
(the header measuring its height, or expanding the brief, both push the content down — someone following the tail should stay at the tail).
The structure is pinned by `hqChrome.test.ts`, the twin of `detailChrome.test.ts`.

### The neighbour-pane bar (tiered-pane-control)

At the top of Detail (below the header, above the segmented control) sits a **horizontal neighbour-pane bar**: the **other panes** of the tmux session this pane belongs to (`GET /api/panes` filtered to the same session).

**Every chip carries an identity icon**: an agent pane wears its official icon, an ordinary pane wears the `$_` letter mark that `AgentAvatar` falls back to —
the same recognition token as the radar row. Earlier it was two glyphs, `▸` / `›`, saying one thing with two symbols the reader had to learn first.

**An ordinary pane needs a name, not a command name** (`api/types.paneLabel`). It used to print `command`, so three shells in one session were all called `bash` — true, but **distinguishing nothing**, and the reader is choosing one of these chips. The naming order is by "what this step says beyond the last":
`title` (someone named it deliberately; the core already drops a title equal to the hostname) →
`win_name` (unless tmux auto-renamed it to the command name, which is bash again) → `project` (which repo it is in; stable across subdirectories, and what people actually call it) → the last segment of `cwd` (when not in a repo) → `command` (still true, just the last resort). Tapping one = open that pane's Detail (any pane can be viewed / typed into; an ordinary pane is adapted via
`paneRowToAgent`). **Hidden when there are no sibling panes or in full screen**; a guest sees only the panes they were granted. On the desktop the "neighbours" are covered by §16's
pane browser (grouped by session); the phone uses this bar — both on the same `/api/panes` contract.

### Terminal rendering (narrow-screen adaptation)

- Data: `GET /api/pane` every ~1.5s. **`/api/pane` uses `tmux capture-pane -e -p`** (with ANSI SGR), so colour is preserved.
- **Coloured output**: the RN side maps escapes to coloured `<Text>` spans with a lightweight ANSI/SGR parser, matched to macOS
  Terminal's "Pro" dark theme: prompt `$` green, command names cyan, commit hashes yellow, PASS/✓/`ok`/diff `+` green,
  FAIL/diff `-` red, `Tool use:` magenta, box lines/selectors dim grey, `❯` selection green, body `#D6D6DA`.
  The palette aligns with `theme.ts`.
- **Narrow screen ↔ wide window tricks**: ② **font size A− / A+** three steps + ④ **scrollback buffer** + a bottom-right **↓ jump to bottom** FAB (**implemented**).
  ① the **wrap / scroll** toggle and ③ a `cols × rows · live` indicator at the top are **deferred**: ① a nested horizontal `ScrollView` on iOS
  goes white (NativeTerm currently soft-wraps at phone width, see its comment); ③ the server's `/api/pane`/`agents` do not yet send the
  pane's real column width/row height, so anything built would be synthetic — a pane-size field (a contract change) has to come first for it to mean anything.
- **A short buffer must not go black**: capture keeps the pane grid's **trailing blank lines** (the server keeps them for the bottom-anchored cursor row arithmetic,
  see `internal/tmux` CapturePaneColor), and the renderer must trim all-blank trailing lines before display (`term.ts renderView`:
  the cursor row is computed on the **untrimmed** array first, and trimming never cuts into the cursor's row) — otherwise a large empty pane (200×50 with
  5 lines of content) or a `clear` leaves the tail-following scroll view parked in the blank region, and the whole screen is black.
- Monospace font; show the last frame while offline.

### Terminal text selection (iOS 终端文本选中)

Selection/copy on iOS is a **native implementation** (Android keeps the flat `<Text selectable>` overlay untouched). After four
failed attempts (RN selectable gives only a menu; a UITextView overlay misaligns and stutters; a full-screen select sheet's estimates drift;
drag works only downward) the decision: **a uniform grid + a self-implemented read-only subset of UITextInput**, so the system selection UI (band +
two-way handles + loupe + Copy menu) draws directly on our own coloured rendering, with geometry entirely supplied by us → zero misalignment.
Design and verification record in `openspec/changes/mobile-native-term-selection/` (device acceptance 2026-08-08).

How it works:

- **Stage 1 · the uniform grid (JS, `term.ts`/`NativeTerm.tsx`)**: every visible row has an explicit equal height
  `rowHeightFor(fs)` (1.6×; Menlo and PingFang CJK share it); logical lines are **hard-wrapped by cell arithmetic** in JS
  (`charCells` CJK=2, selectors/ZWJ=0; `colsFor` capacity is conservative by −1, so a row is never re-wrapped natively by RN Text).
  Row geometry is then pure arithmetic, `row = ⌊y / rowH⌋`, and the char-wrap matches tmux's own wrapping.
- **Stage 2 · the native layer (`mobileapp/ios/TermSelection/`)**: a transparent `TermSelectionView`
  absoluteFill over the row stack implements a **read-only subset** of UITextInput (positions/ranges/`caretRect`/
  `selectionRects`/`closestPosition`…; in-row x↔char uses the Core Text advance of a cached CTLine — the same shaping engine and the same font as
  RN Text, so the advance of a CJK fallback is measured, not assumed to be 2×cell).
  System parts: band+handles = `UITextSelectionDisplayInteraction` (iOS 17+), loupe =
  `UITextLoupeSession`, Copy menu = `UIEditMenuInteraction` (en/zh). All gestures are self-driven: the activating long-press hangs on the
  outer scroll view; while inactive the overlay is invisible to touch (`point(inside:)` false + box-none), so link taps/scrolling are unaffected; during selection JS freezes the snapshot (`onSelectionActive` → freeze/thaw).

Five traps (read before touching this block; every one was hit for real):

1. **The effective font size must fold in Dynamic Type everywhere**: `fs = fontSize × PixelRatio.getFontScale()` (the code takes
   `useWindowDimensions().fontScale`); wrap cols, rowH, the block-row font and the overlay's CTLine font are **all**
   derived from this one number, and block rows additionally set `allowFontScaling={false}` against double scaling. RN Text scales by fontScale by default
   while the grid arithmetic does not → on a device with a text size above Large the rendered stack overflows rows×rowH, and the bottom (scale−1)/scale of the screen
   is dead to long-press (a 1.0-scale sim cannot reproduce it; `simctl ui content_size extra-large` can).
2. **Do not attach UITextInteraction**: its internal gestures clear `selectedTextRange` every frame during a handle drag,
   so the drag breaks (proven in the sim). Band/handles/loupe/menu are implemented explicitly with the three public pieces above; the system handle views also need
   `isUserInteractionEnabled = false`, or the knobs swallow touches and cannot be dragged.
3. **Pass colours across interop as hex strings** (`"#RRGGBB"` prop, parsed natively): `processColor` packs
   ARGB and the new-architecture interop layer decodes RGBA → the blue selection band turns red.
4. **The overlay text must come from the same source as the rendering**: `flattenGrid` produces the row stack and the overlay text from one wrap,
   with the invariant overlay rows == stack rows (unit-tested); the padding spaces spliced in for the cursor come from the RAW line and never leak into
   Copy. Once the row count drifts, every selection below it is misaligned.
5. **Swift only consumes props, never computes**: rowHeight/fontSize/padTop/padLeft all come from JS
   (`rowHeightFor`/`PAD` are the only numeric source); deriving row height natively from UIFont metrics would replay the misalignment of the
   UITextView era.

### Top bar

- Back ‹ + status badge + primary/secondary. (**Focus on Mac was removed** on the phone (#85) — the top bar no longer holds that
  button, matching the mockup; focusing the Mac is still available from the menu-bar app.)

### Input history is scoped by "work", not one global list (2026-09-06)

Changed only after measuring on a real device: 35 projects, 8791 prompts, filtered to lengths a phone would type —
**96% of distinct entries occur in only one project**, and not as one-offs: they recur within that project
(`发布` 29 times, all in the same project). Only 4% genuinely cross projects, and they are the handful
(`continue` / `继续` / `ok` / `/compact`) that belong to **quick replies**, not history.

So a global list of 30 is mostly other projects' words, and **the entry you actually want has been pushed out by irrelevant traffic**.
That is what needed fixing, not "there are unrelated things in it".

- **The scope key is `project`, falling back to the tmux session name.** Both are stable across restarts and both are names you chose,
  whereas a pane id is not (after a tmux restart `%7` goes to another pane). Measured, the fallback covers exactly HQ and the panes not in a
  repo (`vps-audit`, `disk-triage`, `日常更新`) — 5 of 18, including the one you type into most.
- **Two panes of the same repo share one scope**, which is what is wanted: they are the same piece of work.
- **This scope first, then topped up with cross-scope "recent".** So a brand-new project **is never emptier than the old global list**,
  and the entry you want is **at the top**. The top-up doubles as the upgrade migration: the old flat array becomes "recent" unchanged, and the first look after upgrading is the same as before.
- **Delete by text, and from both sides.** The list is "this scope + top-up" stitched together; the 4th item is a position in the rendering,
  not in storage; deleting from the scope alone lets the top-up push it back one row lower, which reads as the delete not working.
  Clear clears everything — clearing only this scope lets the top-up refill it at once, which looks like the button did nothing.
- The rules live in `state/history.ts` (testable); the wiring is `Composer`'s `historyScope`;
  **test both** — storing correctly but reading with the wrong scope shows you someone else's list.

### Half-typed words must survive (drafts, 2026-09-06)

**Unsent input is stored per pane and survives leaving the screen.** Drafts used to be the composer's local
state; going back to the radar unmounted the screen, so "go see what the other one is doing, then come back and finish" cost you
that sentence. On a phone this is especially common, because what pulls your eyes away is usually a notification.

- **Per pane, never global.** A draft is written for that session; showing it in another pane's input box is
  worse than losing it — this box types straight into a live terminal, and a draft on the wrong line is one Enter away from being sent.
  Every pane sees its own or nothing.
- **Expiry matters more than waiting for its pane to return.** tmux pane ids are per-server sequence numbers; after a restart
  `%7` is another pane. So drafts have a TTL (7 days) and a cap (20 panes), trimmed on write.
- **Empty input means no draft**, not an empty draft; otherwise blanks crowd real drafts out of the cap.
- **Refill never overwrites what is being typed.** The read is asynchronous; if you have started typing by the time it lands,
  it stays out — otherwise it would wipe the first few characters you just typed.
- **Demo neither reads nor writes** (the same rule as input history and quick replies): what is in there is what you typed at a real machine.
- The storage rules live in `state/drafts.ts` (testable), the wiring in `Composer`'s `draftKey`;
  each has its own tests, and **the wiring half is the half that actually leaks** — storing correctly but never refilling
  loses exactly the same experience.

### Composer (input · Phase 2, writing needs a one-time grant)

Input has a clear hierarchy, **foregrounding agent-management input**, with free text as the extension:

- **Contextual shortcuts (agent-shaped)**: when waiting, `1·Yes / 2·Always / 3·No` directly; in other states
  `continue / ⏎ / stop`.
- **The control-key row**: `Tab ↑ ↓ ⏎ ⌫ Ctrl-C Esc` (horizontally scrollable). `↑/↓` navigate, `⏎` submits, `⌫` backspaces —
  so interactive TUI pickers (Claude Code's AskUserQuestion, single/multi-select) can be driven in the terminal. Such
  rich pickers **get no one-tap ApprovalCard** (bare digits cannot drive them); the terminal key row is their reply channel.
  (The `␣` space key was removed 2026-08-08: never useful, a literal space can be typed from the input box; `⌫` sends tmux `BSpace`
  to fix a slip on the agent's input line.)
- **Free-text box + send**: any text, as the catch-all.
- Everything goes through `POST /api/send` (send-keys), **gated by write permission**: without a grant the composer is greyed and labelled
  `Phase 2 · writing needs a one-time grant`.

### Voice input

- The microphone key opens a full-screen listening state: pulsing microphone + waveform + live transcript + cancel / send.
- The transcript goes through the same `POST /api/send` as the composer (same write-permission gate).

---

### Knowledge base: "Recent" is a view, not a bucket

"Recent" and "Topics" look at **the same entries**: recent = the first few of all entries by time, each of which also lives in its own
topic, and the topic counts add up to the total in the title bar. **No entry floats outside a topic.** The interface never said so, and a reader looking at "396 entries" beside a list of 6 reasonably asked "do these not count towards any topic"
(2026-09-07) — a sentence under the section now answers it, the same sentence on Mac and phone.

Along the way, a false statement on the Mac side was fixed: the number beside the section title was the **library total**, while only 12 entries were listed below it.
A number must describe what it lists; the total is already in the window title.

**Topics can expand in place**, not only be entered. Both ends use a fold row (`▸`/`▾`); one tap shows the topic's
entries where you are — a glance should not cost a screen change plus losing your place.

**One deliberate difference between the two ends here**: the Mac lists **all** of a topic's entries when expanded (the window is large and scrollable, and the Mac
has no separate topic page); the phone lists **the 5 most recent + "all N ›"**, and the full list is inside.
Spreading 179 rows into a modal sheet on a phone only moves the problem.

**Backing out of an entry returns you to where you were.** Back returns to **where you tapped in from** (a topic page returns to the topic page,
not always to the root), and each level remembers its own scroll position and restores it. Previously, tapping the fourth entry of a 164-entry topic and backing out
landed you at the top of a different screen — the list you were reading simply vanished.

### Attachments: the plus route must be short

`+` is a **four-way** bottom card (Photos · Camera · Files · Paste), with **Photos first** — the phone is the Mac's
remote control, and most of what people send in is a screenshot they just took; Camera was first only because it was written down first.

**After a choice, the card plays no exit animation.** The order cannot change (launching the system picker during a Modal's dismiss animation fails
silently on iOS; "+ → Photos" once did nothing at all for that reason), but the animation standing in the way can go: sliding back takes ~300ms,
during which the input bar fills the screen, and only then does the system picker begin its own slide up. Two full animations to pick one image, with a pointless
trip back to the input bar in the middle — which is exactly how it looked (user feedback 2026-09-07). Closing by tapping the backdrop or the back gesture
**still slides**: there the card's disappearance is the result, and the motion is the feedback.

**A chosen photo goes straight into the annotation editor**, enters the staging area only when editing finishes, and leaves nothing behind on cancel. This was tried the other way round for a whole
version (photo straight into staging, annotation by tapping the thumbnail) and reverted the same day on the user's actual usage: **most** of the images he sends
need annotation, so moving the editor to a second tap saves one tap on the few "send as-is" images by adding one to nearly every
photo. **Defaults of this kind are decided by frequency of use, not by fewest steps.**

That attempt left one thing worth keeping: **thumbnails are tappable**, opening the same editor, and the edit **replaces the original**
(appending would send the same image twice). So re-annotating works, and images arriving by other routes can be annotated too.

Paste also goes into the editor first, but that is unrelated to the interface — the clipboard hands over a data: URI, and the editor is the step that turns it into a file.

### Controls floating over terminal content (exit full screen etc.)

A control floating over **arbitrary terminal output** **cannot borrow contrast from its background**. The exit-full-screen pill once used a translucent fill
(`rgba(20,20,22,0.82)`) + a 0.14-alpha hairline border — the terminal's own text showed straight through, and the control blurred into the background.
The iron rule: **the fill must be opaque**, the border a real line, plus a drop shadow to lift it. Then the same pill reads on dense text,
on a blank panel, and under dark or light terminal themes. Text uses a **fixed light colour** (such surfaces are always dark; `pal.fg`
turns near-black in light mode → invisible, see [[mobile-light-mode-dark-surface-trap]]).

The icon expresses the **action**, not a shape: exit full screen uses `✕` (close), not `⤡` (a diagonal resize arrow — it reads as
"bigger/smaller", not "leave"), and is paired with the full word ("Exit full screen") to leave nothing to guess.

### Full-screen mode = reading mode

Full screen is defined as **maximum reading space**: content only (terminal/chat) — the top chrome **and the bottom
composer/key row** are all hidden, and restored on exit (decided by the user 2026-08). ApprovalCard / the send-failure bar are
**exception-state reminders**, not chrome, and still surface. In terminal full screen the safe-area backing must use **the terminal's own dark**
(`TERM_BG` / the terminal theme's background), never the app theme — in light mode `pal.bg` once painted a bright strip above an always-dark terminal
(the backing variant of the dark-surface trap); the chat's full-screen backing still follows the theme (the chat surface itself follows the theme).

**Full screen must really be full screen** (raised a second time by the user 2026-09-07, 「上沿还是很低，屏效很差」, "the top edge is still very low, poor use of the screen"): in full screen the container
**no longer applies the top safe area**, and content draws up to the physical top of the screen. The status bar is already hidden there, so that ~59pt
safe area was reserving space for a status bar that did not exist; stacked on the content layer's original 42pt offset, a view meant for "maximum reading space"
had ~100pt of nothing pressing on its top. Both are reclaimed: portrait gains about 12% reading height, landscape (where the reading area
was only ~390pt) gains about a quarter.

The cost, stated plainly: the topmost line goes under the Dynamic Island. Acceptable, because the island covers a **scrollable** region rather than a
fixed frame — you read the live tail at the bottom, and the older content in the island's band is one flick away. This is not the 2026-08-09 case:
there the permanently covered first line sat under a **full-width pill with text**; the exit control is now a 28pt
corner mark occupying one corner, and in landscape the cost drops from "a whole line" to "a small piece of a line". The exit control positions itself by
`useSafeAreaInsets().top`, so it still sits below the island. The full-screen backing is painted edge to edge by the outer container, and the island's band
still uses the terminal's own dark, so no bright strip appears.

**In landscape the safe areas are left and right, not top and bottom**: with `SafeAreaView` taking only `top`, the notch/island moves to the side in landscape and
both terminal and chat slide under it. Take `['top','left','right']` — in portrait the side insets are 0, so this only affects landscape.

## 5. iPad / tablets and adaptive layout (rewritten 2026-09-12)

The iPad **is not "a bigger phone"**: the large canvas takes the whole picture in with "sidebar radar + main area". This section was first written in 2026-07 and half built
(`SplitScreen`, `ui/layout.ts`), then deferred for the first store release; the target was always iPhone-only, so it never reached a device.
The basis for this rewrite: that half implementation was a copy of RadarScreen, and every radar change over two months (the floating HQ disc, the errored section,
the guest banner, the long-press menu) landed only on the phone. Design canvas: https://claude.ai/code/artifact/b4c7d610-88cb-4b2a-b58f-18dbe789b914 ;
decision record: the design.md of openspec change `ipad-universal-app` (D1–D11).

### One rule, two shells

- **`regular`**: window width ≥ 768 and height ≥ 600 (`isSplitCanvas` in `ui/layout.ts`, unchanged) → sidebar + main area.
- **`compact`**: everything else → the phone's stacked navigation, unchanged to the letter.
- iPad 1/2 split (683 wide) and Slide Over (320) are compact, deliberately; 2/3 split (910) is regular.
  iPhone landscape at 852–932 wide and 393–430 tall fails the height line: **the iPad is not a bigger phone, and a phone in landscape is not a smaller iPad**.
- Screens do not read the window size to pick a layout; only the shell does (`useSizeClass()`), and a screen receives a `layout` prop.

### Layout (regular)

- **Sidebar**: 300pt when ≥ 1000 wide, otherwise 280pt. Its content is the phone's radar: brand + server name + connection dot + one summary line;
  §3's collapsible sections and section dividers; **the selected row** uses the `rowSelected` ground + a 2.5pt cyan bar on the left; at the bottom the HQ card
  (a persistent sidebar uses a card, not the floating disc) and the "all panes" entrance.
- **The sidebar can be collapsed** (top-left button or ⌃⌘S, remembered). The 2026-07 version said "in portrait or a narrowed split the sidebar becomes a ☰ drawer";
  this rewrite rejects it: an 11" portrait is 834 wide, and a 280 sidebar leaves the main area 554, wider than any phone; a drawer is a second layout raised for a problem
  that does not exist in the numbers. "I want the whole width" is served by the collapse button, which also covers 768–1000 windows in Stage Manager.
- **Main area**: shows one of the currently selected things — a pane's Detail, the HQ page, all panes. Tapping any sidebar row switches in place, no page push.
- **Reading width**: chat, the HQ conversation area and Settings cap at 760pt and centre; **the terminal is never capped**, more columns being the point of a big screen.
- **Form-like sheets** (board / usage) keep pageSheet, which iPadOS itself centres as a form sheet;
  **the knowledge base in regular is a two-column "list | body"** (like the menu-bar window, §4 "The menu bar's knowledge base").

### No copies: the sidebar is the phone's radar

The phone's radar screen and the sidebar render **the same** `RadarPanel`; the difference is props only: how a row opens (push a route / select in place),
which row is selected, width, and whether the HQ entrance is a card or the disc. Detail was already shared (`DetailView`); the HQ page and all panes split into
"a route shell (compact: back button, safe area) + a view (used by both shells)". `SplitScreen.tsx` is deleted.
One structural test stands guard: a second radar shell, a screen picking its own layout from the window size, or `SplitScreen` coming back turns the build red.

### "What is open" is workspace state

The currently open pane / HQ / all panes sit in one `WorkspaceContext`. The compact shell translates it into navigation,
the regular shell renders it into the main area. Push deep links, the HQ page's "Open session" and keyboard commands only change this state and know nothing about either shell.

### The HQ page (regular)

The report header (verdict sentence, standing line, three doors) spans the main area; below it the conversation area on the left (capped at 760) and a 360pt **inspector** on the right holding the two sections
the phone hides behind tabs: "Your call" (decision cards, then what is running) and "HQ's work". The quick chips and the input box stay at the bottom of the conversation area.
The canvas's second page records the two other directions and their cost: B stacks both sections above the conversation (everything visible at once, but the conversation loses 150pt per screen and the two sections
scroll together); C keeps the phone's tabs, just wider (no new layout to maintain, but a 1194pt screen shows one section, and the things needing your call stay hidden behind a tab).

### iPad extras

- **Hardware keyboard**: one native bridge (`UIKeyCommand`) + one table (`src/keys/keymap.ts`). ↑↓ move the radar selection, ⏎ opens,
  ⌘1–9 jump to row n, ⌘⇧H HQ, ⌘⇧P all panes, ⌘F search panes, ⌘K focus the input box, esc closes a sheet, ⌘[ ⌘] chat/terminal,
  ⌘= ⌘− font size, ⌃⌘S collapse the sidebar. The system's hold-⌘ overlay is generated from the titles. The table is in `src/keys/keymap.ts`, and a test pins that every binding is present.
  **The simulator cannot verify dispatch**: XCTest's key injection only types, it does not dispatch UIKeyCommand; whether the bridge receives anything can only be checked by pressing a real keyboard once.
- **Pointer**: rows and buttons tint with `rowSelected` on hover; no custom cursor.
- **Multitasking**: `UIRequiresFullScreen` is not declared; Split View / Slide Over / Stage Manager are all allowed, and the shell follows the window.
  Multiple windows (scenes) and Apple Pencil markup are out of scope this time.
- **Orientation**: all four supported, not locked.
- **Store**: the 13" iPad screenshot set (landscape 2752×2064, `frame-shots.mjs --slot ipad`) is drawn by the phone's demo-mode pipeline on an iPad Pro 13"
  simulator, in both languages (`GTMUX_DEBUG_LANG` forces the language); the lock-screen shot exists only for the phone (a Live Activity has no iPad presentation).
- **Demo is the same shell**: on iPad, demo mode is sidebar + main area, and App Review sees what users see (SURFACES.md §3).

---

## 6. Visual rules recap (consistent with the menu bar)

- **Status**: waiting `#EF4444` red square·double bar / working `#06B6D4` cyan circle·loading ring (**slow rotation**, 2s per turn, respecting Reduce Motion; DESIGN §10) /
  idle `#22C55E` green circle·check / running `#8E8E93` grey circle·dot.
- **Section order**: needs-you → working → idle → running; the waiting section title is red, the rest neutral.
- **Dark/light** (theme.ts): dark `bg #0D0D0F · surface #1C1C1F`; light `bg #F2F2F7 · surface #FFF`.
- **i18n**: en/zh follows the system, lockable in Settings; CJK truncates with an ellipsis, never wraps.
- **Motion**: only the single idle→waiting pulse; everything else quiet. No gratuitous gradients, no glowing shadows, no marketing copy.
- **English capitalisation has exactly three registers, decided by "what this text is doing", not by mood** (settled 2026-09-05; before that
  the three tabs were `your call` / `what HQ did` / `console`, with `Situation board`
  and `Activity` on the same screen — the user's verdict was 「大小写感觉很随意不专业」, "the capitalisation feels arbitrary and unprofessional"):
  1. **Sentence case — every "name" and "action"**: section/tab titles, buttons,
     chips, sheet titles, settings rows, group titles. `Your call` / `HQ's work` / `Console` /
     `Mark it landed…` / `Open session`. Proper nouns keep their form (`HQ` / `gtmux` / `Tab`).
     **Capitalise the first word only** — Title Case demands a per-word "is this important" call, which is exactly where arbitrariness comes from.
  2. **All lowercase — the status words themselves**: `waiting` / `working` / `idle` / `running` / `errored` /
     `offline` / `reconnecting…` / `read-only` / `landed`. This is the status language shared by three surfaces;
     **a status is not a name**, and capitalising it promotes it to a title.
  3. **Small all-caps (eyebrow) — the key beside a value**: `FLEET` / `USAGE` / `BOARD` /
     `WAITING ON YOU` / `NEEDS YOU`. **Generated by the `textTransform` style; the source is written lowercase**
     (the menu bar likewise, `.uppercased()`), so it is always a visual tier and never a spelling choice.
  Chinese has no case, but **one group of labels must share a part of speech**: three tabs are either all noun phrases or all
  questions, never a mix of 「该你拍板 / 参谋长做了什么 / 对话」 (your call / what the chief of staff did / conversation). Menu-bar DESIGN §17 is the same rule.

---

## 7. Push and connectivity

- Path: `gtmux serve → push relay (holds the APNs key, stateless) → APNs → device`. iOS uses native
  APNs tokens, **no Firebase needed**; the token is stored on the Mac via `POST /api/push/register`.
- Two kinds of `alert`: `waiting` (any state→waiting) / `done` (working→idle). In the foreground these become in-app banners.
- **Tapping a push → deep-links to that pane's Detail** (reading `pane` from the payload).
- APNs is delivered by Apple, **received even off the VPN**; only live pulls / focus need the internal network.
### The lock-screen card and Dynamic Island (Live Activity) · redone 2026-09-10

**It answers exactly one question**: is anything waiting on me, which one, what did it ask, and for how long. The card's top line used to be
"2 waiting · 3 working" — a number, which cannot answer the last three; and the answers (the session name +
the agent's own words) were in the pushed data all along, surfacing only in the fallback branch for "no session can be listed at all".

- **The main column goes to the one waiting on you**: the session name at 16pt bold, its question on one line, and the waiting time via `.timer`,
  ticking locally (no push budget spent). When nobody is waiting it becomes "N running / longest for", and when all is idle one line, "All quiet".
- **The card's ground follows the state** (a very faint red / cyan / green / grey gradient). It is a **backing, not an encoding** —
  the state is still carried by the status mark with DESIGN §1's colour + shape + glyph.
- **The height budget is 160pt, and it is hard.** The implementation measures about 136pt. The first draft of this version ran to 206pt, which a device
  simply clips. **Every line added here must be taken from somewhere else** — so the question gets one line, only two sessions are listed below it,
  and "and N more" goes to the count strip at the top rather than another line.
- **The server name is the card's identity, not a footnote.** gtmux connects to several Macs and switches between them (`App.tsx` keys on
  the Mac's url, and a switch remounts the whole thing: end the old card, open a new one under the new name). Two Macs are two cards that look
  much alike, so the name walks the first line with the status dot instead of shrinking into small grey text.
- **On disconnect everything drops to grey**, the timer changes from counting up to "how long ago", and a sentence says why it stopped.
  A red card still counting upward says the opposite of the facts.
- **Dynamic Island, compact**: the status mark on the left; on the right, when someone is waiting, **the waiting time** (the mark on the left already says
  how many, and "waiting four minutes" is the number that decides whether to take the phone out), otherwise back to the count.
- **The loading ring does not rotate, and that is not an omission.** Measured on a device 2026-08-13: `.repeatForever` rotation
  simply does not run in a Live Activity; WidgetKit renders timeline snapshots only. The motion comes from those locally ticking timers.

- **The card speaks Chinese too (2026-09-11).** The card had not one word of Chinese — the whole app is bilingual except the lock screen,
  which is precisely the side users see without opening the app. The widget extension cannot read the app's `GTMUX_LANG`;
  it follows the **system language** (`Locale.preferredLanguages`), which for a lock screen is how it should be anyway.

### The store's lock-screen screenshot is drawn, not captured

Live Activities and push notifications are two of this product's core actions, and the store page showed neither — because neither **can be captured** here:
a simulator build has no `aps-environment` (simulator builds carry no entitlements, even when ad-hoc signed), so ActivityKit
refuses to create the activity; `simctl` cannot reach the lock screen either. Capturing on a device would put the operator's own session names in the picture, which is the whole reason the demo exists.

So `scripts/render-lockscreen.mjs` **draws from the widget's source**: every size, colour and string is taken at 3× from
`ios/GtmuxWidget/GtmuxWidget.swift`. **When that source changes, this picture must change with it** — a drawn picture
has no way to verify itself, and pretending it can is worse than saying so here.

### Why the card was "late" (fixed 2026-09-10)

The pipeline itself is healthy — measured: the Mac recomputes every 1.5 seconds and pushes only on a real change (about once a minute for this fleet),
the relay delivers every one to APNs and gets OK back, `NSSupportsLiveActivitiesFrequentUpdates` is on,
priority 10. **What was missing was "re-sending"**:

- **Push the current state the moment a token registers.** A freshly created card (app restart, phone restart, switching to this Mac) used to
  wait for the fleet's next change.
- **Push once more when the phone reconnects.** It has just come back, and the card most likely shows the version from before it left.
- **Push failures are no longer swallowed.** When APNs says 410 (dead token), forget it — switching a phone between two Macs leaves exactly this kind of dead token,
  and the Mac switched away from would push at a non-existent card forever; other failures are counted so doctor can see them.
- **The fallback heartbeat 30 minutes → 5 minutes.** It is also the recovery floor: lose one push and the card is stale for at most that long.

- Pairing QR schema v1: `{ "v":1, "url":"https://host:port", "token":"<serve-token>", "name":"…" }`.

---

## 8. States and edges

| Scenario | Behaviour |
|---|---|
| 0 agents | empty-state card "no coding agent running", no error |
| 1 waiting | lands in "needs you": red square·double bar + faint red ground + one pulse |
| 15+ | the list scrolls; the "waiting only" filter narrows it |
| very long task / CJK | single-line ellipsis truncation, never wraps/overflows |
| offline / reconnecting | offline red dot / reconnecting; pushes still arrive via APNs |
| idle→waiting | green circle→red square, one pulse + alert |

---

## 8b. The What's New popup

Shown **once** after an update, the equivalent of the CLI's `gtmux whatsnew`; viewable again any time under Settings → About
(release notes you can only read once are notes you cannot read).

**Spanning versions is the core scenario**: a user who skipped three versions must see all three versions' notes, not only the newest.

- **Older versions fold (whatsnew-fold-older, 2026-09-14: 「whats new 会越来越多，比较旧版本的信息应该默认折叠」).** The newest version is open; every older one is a heading with its item count and a chevron, and opens in place — Settings included. This replaces the eight-item cap below: a reader who skipped versions still sees that they exist and how much each changed, and the card stays one screen however long the archive grows. The cap's three rules are kept for the record.
- **Headings and items are laid out as written (2026-09-15).** The store notes since 1.0.17 are headings with `- ` items under them; the popup drew every line as a bullet, so each item showed as "• - The same app…". `state/whatsnew.noteItems` reads the structure: a heading carries no bullet, an item is indented under it, and a release without `- ` lines stays a flat list.
- **Two tiers, isomorphic with the CLI** (superseded by the fold above; kept for the record):
  - The popup = a summary. Grouped by version, newest first, **truncated to 8 items** (the CLI's `changelogMax` is 5; here it is
    a card the user is actively reading, and 8 lets a common single-version release (5–6 items) show in full, with folding appearing only when "you really did
    skip versions"). The fold shows "N more — show all" and **expands in place**, no page change.
  - Settings → What's New = the full set, expanded.
- **Three rules for truncation** (all in the spec): a version shows in full or folds in full ("3 of 0.46.0's 6 items"
  means nothing to a reader); the fold is always a **suffix, never a hole** (skipping a middle version to fit a smaller one
  tells the reader that version changed nothing); **the newest version always shows**, even if it alone exceeds the cap.
- **The copy comes from the per-version archive** `mobileapp/release-notes/<version>.{en,zh}.txt`, compiled by
  `scripts/gen-release-notes.sh` into `src/releaseNotes.ts` (newest first). **App Store metadata cannot be the source** —
  it is overwritten every release, leaving only the current version. `set-version.sh` archives the current store copy under the new version number when
  the copy actually changed, so the normal release flow needs no extra step. `check-design.sh` regenerates and compares byte for byte; a mismatch is red.
- **Language** follows the three-state language setting (system / EN / Chinese); a missing language falls back to the other — the same fallback rule as the CLI's
  `user:` / `user-zh:` blocks in a tag. The truncation count is taken in **the language being read**.
- **Versions are ordered by numeric segments** (0.10 after 0.9); archive entries newer than the binary are not shown.
- **No popup on first install**: with no old version seen, nothing is "new". Record the version silently and let **the first update** be the first greeting.
  Versions with no archive content likewise do not pop (a CLI-only release may genuinely have nothing to say to phone users).
- **Visuals** (§6 iron rules): **no NEW/ENHANCED/FIXED columns, no emphasis fills, no motion** — release notes are
  read once and closed, and dressing them up is the forbidden marketing register. A centred card, not a full-screen page; the backdrop or the button both close it. Items
  are prose and **wrap rather than ellipsise** (§8's CJK ellipsis rule is for list rows).
- **Brand comes from the product's own vocabulary, not decoration**: the card header holds `BrandMark` (the app icon's pane grid),
  version numbers are always `Menlo` (the same monospace as the terminal, branch chips and HQ figures — a version is a token, not prose),
  and the bullet is **a pane cell** (a 5pt rounded square), not a typographic dot. That is all.
- **The version number does not appear twice**: when group headings are present, the header's version chip yields; with a single version, the reverse. Printing both is
  not hierarchy, it is noise.
- **Never wrap a `ScrollView` in a `Pressable`.** The backdrop was once the card's **parent** (outer tap closes +
  an inner empty `onPress` blocks pass-through), so a touch was claimed by the JS responder at start and native scrolling never got the gesture —
  the card would not scroll / stuttered (user feedback 2026-08-09). The correct structure: the backdrop is the card's **sibling** (`absoluteFill`
  behind it), and the card is a plain `View`. A test asserts directly that no `ScrollView` in the component tree has a `Pressable` ancestor.

---

## 9. Roadmap

- **MVP**: read-only monitoring + focus + push (covered by this design).
- **P2**: terminal input via `POST /api/send` (send-keys, write-permission gated).
- **P3**: voice.
- **P4**: Android / HarmonyOS (RNOH; components stay platform-neutral).


---

## The HQ command page (HQScreen) · §17

Tapping a `role:"supervisor"` row lands on the dedicated HQScreen (not an ordinary Detail).

**Iron rule: the HQ page no longer lists the fleet row by row.** That list belongs to the radar, one step back; the old "fleet situation board"
merely shrank it into the space above the chat, so its answer to "I am redundant" became a fold toggle that, folded, left an empty bar
(`hq-command-page` fixes exactly this). The fleet **count** stays in the status strip; the **list** belongs to the radar.

The page answers only the three questions the radar cannot, built from **what only the chief of staff knows**:

1. **The permanent header is two lines** — `‹ gtmux HQ ● ` + **one verdict sentence** (turning amber when someone is waiting on you). That is all.
   This was once four bands (status strip + fleet count/subscription % + verdict + a board row), about 200pt by the stylesheet, about
   260pt with the safe area; with the keyboard up, the conversation below had four or five lines left — and most of those 200pt bought the fleet count and
   disk/memory, **one swipe away on the radar**. **The most expensive pixels cannot be sold to what is visible one step back.**
   - The verdict sentence expands (`▸`), and the expansion holds **the chief of staff's own latest brief** (the newest `⟣` reply in the transcript,
     marker stripped) — it already has a 10-minute cadence brief, and **its own words beat a count gtmux recomputes**. With no brief
     this block does not appear; nothing is invented. The expansion also holds the fleet row, the resource row and the board entrance (nothing that exists today is lost; it just no longer charges permanent rent).
   - **Resources rise to permanent only at the red tier** (`verdict` already has a `resource` state). The old version printed disk/memory unconditionally,
     so it read as noise — **a line that is always there says nothing when it really should speak.**
   - **`⟣` belongs to the chief of staff alone; the verdict sentence does not wear it.** The verdict is computed by gtmux itself (`hqZones.verdictSentence`),
     and it used to print `⟣` as well, so one mark labelled two voices one line apart — the passage the chief of staff actually wrote, inside the expansion,
     read like a continuation of the line above. **The register mark is the only sign of "who is speaking" and cannot be lent out.**
   - **The verdict sentence and what it expands into must be one object** (2026-09-05: 「这一条让人感知能点击的感觉太弱了」, "this row barely feels tappable").
     Before, it was a sentence between two hairlines with a 12pt triangle in the dimmest grey at the right end — that is a status band,
     and readers read it as one. Now **the verdict + its expansion form one rounded surface card**, the verdict as the card head,
     the arrow **the same `›` the whole app uses** (rotated 90°), its weight following the sentence it belongs to, not "the faintest grey available".
     **A disclosure control that does not look like the thing it opens gets read as a line of explanation.**
   - **The quote is the chief of staff's "latest sentence", and when that is routine bookkeeping, nothing is quoted**
     (2026-09-05). Measured over one whole session: `▪ noted:` 41, `✅` 7, `⚠` 2, `◈ 简报` (brief) 1 —
     so "take the latest `⟣` reply" printed bookkeeping like "…watermark at 32134" in the header four times out of five, signed as
     a "brief". But **skipping back past the bookkeeping to find something "interesting" is worse**: that `▪` is often exactly the thing that withdraws
     the alert above it (real case: `⚠ 可能要重开` (may need a restart), three minutes later `▪ noted: 压缩成了…上一条升级撤回` (compaction succeeded… previous escalation withdrawn)),
     and a header that skips it keeps an hours-old, already-void alert hanging under an "all clear" verdict.
     So: **the latest sentence, or nothing.** The level comes from the chief of staff's own register marks
     (`⚠` escalation / `✅` done / `◈` brief may stand; `▪` ledger and `📓` distillation may not),
     and **the byline states the level** (`⟣ 参谋长 · 升级 · 3h前`, chief of staff · escalation · 3h ago) — the reader must know which kind they are reading.
   - **A brief needs a source and a time, and only a glance's worth** (byline + body capped at 3 lines + at most 3 `· ` items).
     A passage with no byline and no time is not a brief, it is a passage; and **the full text is already in the conversation below**, so printing it again in the header
     puts the same words on one screen twice. The time is the turn's timestamp; if unavailable, none is shown — **never invented**.
     The leading `简报 14:30 │` of a brief is stripped — the byline already says who and how long ago.
   - **The chief of staff writes markdown; the header either renders it or strips it, never prints the markup itself** (``` `%19` ``` was once shown
     with its backticks). Paired backticks render as monospace; a lone one stays as is — guessing where it ends would silently restyle the rest of the sentence.
   - **Everything in the expansion is a row of one table: a key column, a value column, five rows straight down** (2026-09-05:
     「展开后的信息呈现太乱了」, "the expanded information is too messy"). Before, three layouts were stacked — a quotation, a key-value table,
     two "icon + text + arrow" rows — each with its own left edge. Now **the board and the knowledge base are rows 4 and 5 of that table**
     (keys `BOARD` / `KNOWLEDGE`, a `›` at the right meaning you can go in), and the two decorative icons `▤` / `◆` are gone —
     **when a row's key already says board, it does not need a drawing of a board.** Values no longer repeat the key either
     (`situation board · 1m ago` → `updated 1m ago`).
     The key column's width is a **measured value per language** (en 80 / zh 48), not taste: five rows sharing one width is what makes it a table.
   - **The usage door leads with tokens by day** (usage-door-tokens, 2026-09-14: 「展示 usage 的地方都需要类似迭代」): `today 2.8M · week 16.2M`, one fact per line of the tile (`HQHeader.destLines`: a phone tile is ~90pt of text, and on one line the second fact always ended as `w…`, 2026-09-14), so every surface that says "usage" answers the same question first. The tightest window lives in the sheet behind the door, drawn with its reset time; an older serve with no `history` shows the windows alone on the tile. The Mac card's usage row has the width and keeps the window after the tokens.
   - **The usage row compresses to "one window per plan" rather than listing every window** (2026-09-06).
     Once Codex quotas came in, the window count doubled and the row began truncating in the middle of a number (`Fable 11…`) —
     a percentage is the last thing that may be truncated. Now each plan keeps only **its tightest window**,
     labelled with whose it is (`claude wk 18% · codex wk 1%`). The tightest, because this row answers
     "where am I standing right now"; deliberately unlike the alert rule (alerts ignore the 5-hour window, since interrupting you for a window that resets
     itself is noise, but when it is the tightest, showing it is the answer).
     An old serve sends no `agent` field; then windows group by their labels and the row falls back to its old form.
   - **The expansion is a "chief of staff's report", not a dashboard** (redone 2026-09-06). Measured by the page's own criterion
     — "what only the chief of staff knows" — three of the old six rows fail: `FLEET 0 需要你 · 0 运行 ·
     17 空闲` (0 need you · 0 running · 17 idle) is **the radar's arithmetic**, printed word for word in the radar's top bar; `USAGE` and `MACHINE` are
     sensor readings, all in the usage view. **The iron rule says the fleet list belongs to the radar and the count to the status strip, and the count had crawled back
     onto the most expensive pixels of this card.** Worse, the order was inverted: the largest and blackest was "all clear" — zero information when true,
     and true most of the time; the smallest, faintest and last was "8 waiting for you to take" —
     a debt with a clock that **only you can repay**; and the loudest colour on the card (amber) went to a disk alert you cannot act on from a phone at all.
   - **So the expansion gives three rows in the order of a chief of staff's report**, each key being the question it answers:
     - **Owed (owed)** — the debt only you can settle (knowledge-base entries waiting to be taken, with the oldest age). **It leads**,
       because it is the only row on the card that is "actionable and nobody else can do it". **Only it may turn amber**, and only past
       `gtmux doctor`'s line (about two weeks) — work in the queue is normal.
       "An agent is waiting on you" is not merged in: the verdict already said so in the attention colour, one line up.
     - **Did (did)** — what the chief of staff did in the last day, taken from **the same** tally as the "HQ's work" section, so the two cannot disagree.
       **This row is the piece the old version lacked entirely**, and the reason it read like a dashboard: **a chief of staff that acts for you but
       does not show you what it did is just a gauge with a chat box.**
     - **Context (context)** — fleet + usage + machine pressed into one row, shown **only when things are not ordinary**:
       counts only when someone is waiting or running (with all idle, those three numbers repeat the verdict above), usage only for **the tightest
       window**, the machine only when the core has actually alerted. It is the least urgent row, so it is the only one
       **allowed to wrap to two lines** — a truncated reading is worse than a wrapped one.
   - **A row with nothing to say is absent entirely**, never `— `. When the red-tier machine rises to permanent, **it is not repeated below**
     (the same number in two places reads as two things).
   - The three registers (quotation · figures · document doors) are separated by hairlines, not spacing.
   - The placement rules live in `hqHeaderModel.ts` (testable); the view `HQHeader.tsx` only draws;
     each has tests (`hqHeaderModel.test.ts` pins the rules, `HQHeader.test.tsx` pins the dividers).
2. **Three sections (segmented switch, each filling the whole body, never squeezed onto one screen; in the regular shell the conversation is permanent and the other two sit in the right-hand inspector, §5).** The three labels are **three peer noun phrases**
   (`Your call` / `HQ's work` / `Console` · `该你拍板` / `参谋长动作` / `对话`),
   not "a noun + a question + a noun" — a label **names a place**, it does not describe it (§6 case rules).
   - **Your call** — one decision card per `waiting` session: status square · window number · session name · agent · waiting time,
     **the ask as the card body**, with two actions below: `Open session` · `Ask the chief of staff`. Selecting one makes it the chips' target.
     Empty state: "Nothing needs your call right now."
   - **(Retired as a zone, 2026-09-15 — hq-work direction A.)** The acts below now sit in the Console as small rows between the bubbles, each above the first turn that came after it (`ui/consoleActs.placeActs`), three or more same-verb acts in an hour folded to one row; the header's "HQ did" row keeps the tally and leads to the console; the tab row is two tabs. The commander's reading (「用途价值不明」) and the analysis are in `docs/design/mockup/hq-work/`: the tab said in a fourth place what HQ's words, the header row and the Mac card already said, with nothing to act on, and an audit is read beside the claim it checks. The paragraph is kept for the record.
   - **HQ's work** — **the chief of staff's own actions** (`gtmux:audit:*`): dispatch / reap / bookkeeping / self-check /
     rotation / handover / alerts, each with its object, content and outcome, under a **weekly tally**. The fleet lifecycle ledger **demotes to
     a filter beside it** (`参谋长 | 舰队`, chief of staff | fleet).
     **Why this is the focus of this version**: measured over a week, the chief of staff dispatched 27 times, reaped 4, kept 168 ledger entries, self-checked 8 times —
     **none of it visible in any interface**, while this section was showing "who started, who finished". **A chief of staff that acts for you but does not show
     you what it did is just a dashboard.**
     - **An act reads as a sentence and leads somewhere** (hq-acts-readable, 2026-09-14: 「不太 get 到要如何理解、利用这些信息，而且这些信息只展示两天的历史」). The rows were the journal's own verbs (`hit pitfalls/… ×2`, `supersede a → b`), pointing at nothing, under no sentence saying what the section is for; and the feed read 24 hours while the tally said "this week". Now: one sentence above the tally says what this is (what HQ did on your behalf, to be checked, nothing to handle); each knowledge act is worded (记下一条 / 改写 / 又踩到 / 晋升给 / 落地 / 退休…) and opens the entry; a dispatch or reap opens the session; outcomes are words (已送达 / 被草稿挡住); the core's acts-only read looks back seven days so the week is a week. Rules in `hqActsModel.ts` (`knowledgeAct`, `outcomeWord`, `purposeLine`).
     - **Wake deliveries are not actions**: that is gtmux knocking on its door (1532 a week vs 39 actions), and counting them buries the actions.
       Channel failures stay visible — as the `wake-degraded` alert, which is the part worth hearing.
     - **The tally order is fixed, not by frequency**: the most frequent (ledger 168) is not the most important (dispatch 27), and rows that reorder whenever
       a number changes must be re-read every time. Alerts first.
     - **Unknown action types still get plain words** (last segment, hyphens opened), never `gtmux:audit:xxx` on the face of the page.
     - **The timeline needs rhythm**: in a column of evenly spaced rows, six quiet hours and a two-minute interval look identical
       (measured feedback 2026-09-10). **Scaling the spacing is wrong** — six hours would take half a screen, and the same feedback
       asked for a compact screen. So **state the gap instead of drawing it**: actions close together collapse into a cluster (threshold 20
       minutes), and between clusters sits one row, "6 h 11 min". A gap takes a row only when there is one; the reader gets the number instead of estimating
       distance. A thin rail on the left strings the entries of a cluster together and **breaks** at a gap — the broken rail is itself the sentence.
     - **Details can expand, but the entrance appears only when really truncated**: an "expand" with nothing more behind it is worse than no entrance. So
       the row text measured via `onTextLayout` is compared with the original, and only a shortfall counts as truncated (`detailTruncated`); when the platform gives
       no per-line text, "hit the line cap" is the only test — better one empty expansion than text hidden with no way to find it.
     - The judgement and aggregation live in `hqActsModel.ts`, the view in `HQActs.tsx`. **The action stream is filtered core-side**
       (`/api/hq/events?acts=1`) — client-side filtering after the fact sees only 3.9 hours (the 200-record cap eaten by wakes).
   - **Console** — the conversation with HQ (ChatView). **A clear is a seam, not a wall** (hq-console-history, 2026-09-15: 「每次只能展示上一次 clear 后的一点内容」): HQ starts over often, each start a new session log, so the console showed only what came after the last one. The serve stitches the session before it on request (`?earlier=N`, following the `hq-session` audit chain), the seam reads 「— 新一段对话 · 23:03 /clear —」, and 「▴ 载入上一段对话」 sits above the oldest turn while one more is known; each tap is one more hop. A worker's Detail has no chain and offers nothing. **The process must be visible while it runs**:
     - **The running turn's steps expand by default**; history stays folded. **Watching and archaeology are two different things**, and they used to share one
       11.5pt fold toggle in the dimmest grey — so there was nothing to watch while it ran, and it was hidden once there was. A tapped one follows the user
       (rules in `ui/chatSteps.ts`).
     - Before a single character arrives, "thinking… 42s" still shows. **It must carry a duration** — "working" cannot distinguish "thinking" from
       "hung", and only the latter deserves an interruption. Without a start time write only "thinking…"; **never invent a duration**.
3. **The command deck is permanent** — chips + Composer in all three sections. With a decision card selected the chips become `帮我回复`/`看它在干嘛`/`让它继续` (reply for me / show what it is doing / let it continue).

**Usage (UsageSheet) · §17.2** — entered through the header's `用量 ›` (usage) door (beside the board and the knowledge base).
It exists because when the redo above pressed three sensor rows into one, the justification was "the details are in the usage view" —
**and the phone had no usage view at all**: the radar reads `/api/usage` only to colour the HQ disc,
and the header's USAGE row was the **only** place on the phone quotas could be seen. That justification was right for the header and wrong for the phone;
this page is the missing half.

- **Quotas group by agent, the name said once**, in the app-wide spelling (`Claude Code` / `Codex`),
  and window rows are called what they are (`session` / `week (all models)`; in Chinese, worded from the serve's `kind` as `会话` / `本周（全部模型）`, with the reset written as a local date from `reset_unix`, and the machine's warning from `warn_key`, 2026-09-15: the agent prints English and a launchd serve has no language, so the phone words all three itself). It used to be flat, every row repeating
  the registry's **lowercase key** (`claude session`, `claude week (all models)`),
  while the session rows lower on the same screen spelled the same agent `Claude Code`.
- **Icons are for identity only**: quota groups and session rows wear the agent's real icon (taken from the radar row; `/api/usage`
  itself carries no icon hint), falling back to the neutral letter mark. The machine rows' icons are **status glyphs**
  (`⚠` over the line, `·` normal), because DESIGN §1 requires status to be triple-encoded as colour+shape+glyph,
  and those rows had colour only. **No icons added for looks** — the keys already say disk/memory/load.
- **"Tokens by day" became the year at a glance (usage-activity, 2026-09-15: 「现在只有一周，信息不紧凑，而且感觉不够丰富」, with GitHub's contribution graph as the reference).** Three figures (today · this week · all since the ledger's first day), a stats line (peak with its date · streak and best streak · daily average · active days), then one of three pictures of the same series behind `按天 / 按周 / 累计` chips: a calendar heatmap (20 weeks on a phone, 44 on an iPad; weeks across, Monday to Sunday down, months above, today ringed, a tapped day read out beneath with its per-agent split), weekly bars (the running week ringed, not inked darker: the greens carry magnitude everywhere), and a cumulative line. The greens are GitHub's five-step ramp, the commander's choice so the picture reads the same as the one everyone already knows; it is a chart, not a status, and the legend says 少 / 多. The Mac reader draws the same block at 44 columns and `gtmux usage --activity` in the terminal, all from one `history.activity`. Canvas: `docs/design/mockup/usage-activity/`. An older serve keeps the seven-day bars below.
- **"Output so far" became "tokens by day" (usage-daily-totals, 2026-09-14).** The commander pointed at the block and asked for the sum everyone actually wants: today's and this week's tokens across every agent. Two hero figures, a seven-day bar chart (one neutral series — colour is status only — thin bars, today's in the stronger ink, direct labels on today and the tallest day, weekday initials beneath, no legend for one series), then the week's split per agent. The core attributes each message to the local day it happened, so the sentence below no longer needs saying. The old bullet is kept for the record:
- **"Output" was not a billing period, and the line under its title said so.** That number is the sum of output across this agent's
  **running sessions, each counted from its own start**: a session three weeks old contributes three weeks.
  It has nothing to do with the "quota 27%" block above, and the layout would suggest it does — so that sentence must be there.
- **The order copies `gtmux usage`**, so the CLI and the phone answer the same question in the same order:
  ① **quota** (the number no local arithmetic can produce) → ② **who is burning it** (per-agent totals first, then the sessions themselves)
  → ③ **the machine** (the one thing that can stop all of the above at once).
- **Sessions sort by "trouble", not "size"**: those the core alerted on first, then by burn rate, then by context share.
  You open this page for the alerted one; a large but idle session is just history. Ties break on pane id,
  so reading an unchanged fleet twice gives identical results.
- **Amber follows the core's tier only**, never this page's opinion that a number looks high.
- **Lead with the conclusion (2026-09-10).** This page was "section titles + flat lists", while the two things the reader wants —
  which window is tightest, and whether the machine is about to stop everything — sat on row 3 and row 25: that day, 18 session rows pushed an
  amber disk alert off the screen. Now a card at the top says both in one sentence; the section order below is unchanged to the letter.
- **Quotas draw as bars, and resets say "how long until".** `9%` and `76%` weigh the same as two numbers; a bar's length speaks first.
  The bar is **neutral grey**: in this product colour means status only, and amber still follows the core's tier alone. The reset time changed from a wall clock
  (`Sep 11 at 10:59pm`) to "resets in 1 day 7 hours", with the wall clock kept after it. **No `reset_unix`, no conversion** —
  that English date string carries no time zone, and parsing it means inventing one on its behalf.
- **Sessions split by "burning or not"**: the alerted and the genuinely producing are listed; the parked fold into one count row.
  **The alerted are never folded**, however quiet — that is the entire point of sorting by trouble.
- **ctx is said once per row.** The core's alert text (`ctx 99%`) and this page's own `Math.round(ctx*100)`
  `ctx 100%` once sat side by side on one row; the subline is freed to say "what it is doing now".
- **The three machine rows carry resource icons (2026-09-10, overturning "no icons added for looks" above).** The icon says only **which resource**
  (a disk platter / a memory stick / a gauge), not whether it is fine; status is still triple-encoded as colour + `⚠` + wording, DESIGN §1 still holds,
  only the "shape" channel moved from the icon to the `⚠`. The first memory icon was a chip with pins, which blurred into a gear at 15pt; it became a memory stick.
- **Quota groups no longer learn their names and icons from session rows.** The core adds `agent_name` to the windows in `/api/usage`
  (from the agent registry, the one place names are declared), and `/api/icon` accepts registry keys too. The phone used to learn each agent's spelling from
  **session rows** — so an agent with a quota but no live session could not be learned: the Codex group showed as lowercase
  `codex` right next to `Claude Code`, and the icon request went out with that key, so `/api/icon?agent=codex` returned a flat 404.
- **The door is permanent; the summary is not.** The header's "context" row disappears when nothing is moving,
  and **by design it is dropped when the machine goes red** — exactly when those readings matter most.
  So the door into this page stands on its own, not hung on that row.
- The rules live in `usageModel.ts` (testable); the view `UsageSheet.tsx` only draws.

**Knowledge base (KnowledgeSheet) · §17.1** — entered from the header's expansion (row 5 of the table, `KNOWLEDGE  353 条 · 7 待带走`, 353 entries · 7 to take,
adjacent to the board: **working memory and long-term memory side by side**). On a real device it is 330 entries / 7 topics; **the phone is not a browser**,
it answers three questions in order:

1. **What does it owe me — promotions waiting to be taken come first**, oldest at the top. This is the **only step in the whole knowledge lifecycle stuck on a person**:
   the chief of staff can judge an entry charter-grade and write the brief, but **only the person who takes it knows it landed**.
   Overdue uses the same line as `gtmux doctor` (about two weeks) — two places giving different answers about one queue, and the reader trusts neither.
2. **What did it just learn — the most recent entries.** A wrong entry **does not lie still**: it is echoed into every dispatch, so it repeats.
   Spot-checking what was just written is the cheapest way to catch it, and it is "reading", exactly what a phone is good at.
3. **Where everything is — topics with counts**, empty topics omitted (the six built-in topics exist on every machine; listing them is five empty rows weighing on
   the one with 137).
- **Actions get their exits per reader** (hq-knowledge-engine): the entry page gains a three-axis line (kind · source×count · for whom); an entry waiting to be taken gets buttons by reader —
  HQ / this machine / repo are "write it in" (gtmux moves it for you, no text, just "now?"), everyone is "feedback to gtmux ↗" (opens a pre-filled issue,
  then "mark as landed" via the link), and one with no reader chosen can only be landed by hand or "withdraw the promotion" (with a reason). `退休这一条` (retire this entry, with a reason) as before.
  Promotion itself is not offered: it asks "for whom", and that question stays on the Mac and the CLI. The judgement is `actsFor` in `knowledgeModel.ts`, testable.
  **Writing entries is deliberately not offered** — typing bodies on a phone is precisely how a knowledge base fills with entries nobody wants to read.
- Actions **ask before doing**: tapping an action only opens the input bar, and sending needs a second confirmation; a failure **shows the server's words verbatim**
  ("has no pending promotion to land" tells the reader what to do; a generic error throws that sentence away).
- An entry without a promotion **gets no land button** — offering one invites a refusal the reader cannot foresee.
- The judgement lives in `knowledgeModel.ts` (testable); the view `KnowledgeSheet.tsx` only draws.

**A copy of HQ's records (Settings → HQ records; "memory" until 2026-09-14)** — HQ's records are the **only irreproducible thing** in gtmux's hands:
a situation board edited for months, a curated knowledge base, a seeded-once-never-overwritten `LOCAL.md`. The Mac now keeps local snapshots,
which guard against accidental deletion, **not against the disk**.

The phone is the answer that **needs no configuration**: it is already paired, and the files in this app's `Documents/` directory
**are already in the iPhone's own backup**. Nothing to set up, no account to create, and the copy comes back with a new phone.

**An honest boundary, said on screen**: iOS has **no** public API for "when did the last iCloud backup run". The app can say the file is within the backup's scope;
**it cannot say a backup ran**. So the same group offers "Export this copy" — save it yourself to Files or iCloud Drive, the version you can
**see with your own eyes**, in one tap.

- **Owner only**: this is the board + knowledge base + `LOCAL.md` packed into one file; a share link only shows someone a pane.
- The row says **how big**, not just a tick: "backed up" and "2.6 MB of irreproducible notes backed up" are two different sentences.
- A failed fetch **reports loudly** (Alert), never silently: a silent failure on a backup page lets someone believe they have a copy they do not.
- Natively, download to a temporary file and **move it into place whole** — a half transfer must never replace a good copy with half of a bad one.

**Rules of the board reader (`BoardSheet.tsx`)** — it is a **long document** (the real board was 842 lines, 48,000 characters when this was written), not a card:
- **The outline must reach the level entries actually live at.** The board's `##` are only two sections, holding 4 and 26 `###`; cut at `##` only and
  the reader sees two rows: folded, a black screen; expanded, a wall of 26,000 characters (the 2026-09-06 report was exactly this). So
  **`##` sections, `###` entries, both levels foldable**.
- **The two-level outline is the "performance optimisation".** Measured on the same board: the whole section 1317 text nodes / 78ms; only the 26 headings
  80 / 3ms; one expanded, 54 / 2ms. **So no loading page here**: a spinner is for work that cannot be avoided,
  3ms is not worth announcing, and announcing it would cover for a render nobody asked for.
- The order is **always the author's**, never sorted (HQ pins "read this first" at the top; sorting would call it the oldest).
  Fold headers carry a **count bubble** on the right: report **this section's own content** first — table rows / bullets — and only when it has none, the entries under it.
  A heading is a promise — under "① 现状 — 在跑的 pane" (current state — running panes) sits a 13-row pane table plus four subheadings;
  counting subheadings turns 13 into 4 and hides the number the heading just asked about.
- **The expansion state is seeded once, on first entry.** The board is polled, a new array every few minutes; resetting with it
  means the entry the reader is looking at snaps shut by itself.
- **The document's own `# title` is not rendered**; the subtitle says only **updated N ago · read-only**.
- **Inline code gets no background block here** (`calmEmphasis`).
- **Tables wider than 3 columns stack row by row** (`stackRows`), and **each row folds on its own, folded by default** (`foldRows`).
  The board's pane table is a table **with paragraphs in its cells**: one pane's "status" column alone runs several screens, and 13 panes spread out
  are something nobody scrolls to the end of, with the pane you want buried inside. Measured on the same board: section expanded, rows folded
  2,490 characters / 138 text nodes; all 13 rows open 24,509 / 637.
  A folded row carries **its first field** (`loc` on the board), or a bare column of pane ids says nothing.
  **The folded row is a "row", not a "card"**: a card is for holding a group of labelled fields, and with the fields hidden it is only a shell around
  a short line; thirteen stacked read like a pile of boxes, not a scannable list; when expanded the card returns and says where the block starts and ends.
  `foldRows` is **opt-in**: tables in the chat are small and part of a sentence, and folding them hides the answer.
- **gtmux supplies the words; HQ does not translate on the fly**: the two sections are fixed as `① 现状` / `② 交接记录` (current state / handover record), the columns fixed as
  `pane / 在做什么 / 谁派的 / 优先级 / 状态 / 等你定 / 教训` (pane / doing / dispatched by / priority / status / your call / lessons; hard-coded since playbook v22). **Give it the words; do not let it translate.**

**Segment labels carry their own signals**: `该你拍板` (your call) carries an amber count badge, `参谋长做了什么` (what the chief of staff did) carries a dot when there are unread **actions** (not unread fleet events — lit for something that is not the section's subject, it sends people to the wrong place) — **the section you are not looking at must
announce itself too**. On entering the page: **someone waiting → land on "Your call"** (that is why you tapped in), otherwise land on "Console".

**The full-screen exit control is an icon, not a pill with text**: four inward-pointing corner marks (`ExitFullScreenIcon`),
**brand cyan `#06B6D4`**, a 28pt button + 15pt icon, positioned by the safe-area top. The icon keeps only the four bare corners: before, each corner
was "corner mark + a diagonal", 8 paths in all, blurring into a blob at 15pt (2026-09-07, 「太丑了，也太大了」, "too ugly, and too big").
The general rule for small icons: **remove strokes, not size**.
No single diagonal arrow — tried and replaced; alone it reads as "resize", not "leave"; no `✕` either,
since `✕` beside a large body of text reads as "close this content", and there is nothing here to close. Brand colour because it floats over a full screen of terminal output and
**must be findable**. The words moved into `accessibilityHint` (`accessibilityLabel` in this file is the automation handle; e2e locates by it).

**Back-to-bottom is "an arrow + a bar", not a bare `↓`** (`ArrowToBottomIcon`): `↓` says
"scroll down"; only the bar beneath it says **the end**, and this key goes all the way there in one press. The colour is **brand cyan
`#06B6D4` for the strokes, not as a fill**: it pairs with the exit-full-screen control at the top left and stays clear of the status language —
cyan means `working` on every surface, and a saturated cyan blob floating on the terminal would be read as a status.

**No section may be "a bare heading + blank space"**; an empty state must be a sentence.

**In the phone's radar, HQ = a draggable floating disc (`HQDisc`)**, no longer a card at the very top. HQ is the **meta layer** (above the fleet),
so it **floats** over the list and does not scroll away, rather than being squeezed in at the top as "another card" (which looks too much like a session).
**Draggable anywhere on the screen**, the drop point persisted (`AsyncStorage`, remembered across launches); tap = enter HQ, drag = move (distinguished by a displacement threshold).
Inside the disc: the gtmux brand mark **+ an "HQ" letter mark** stacked (the logo alone does not point clearly enough; the mark names HQ, the logo stays).
The disc cannot hold the synthesised headline, so **the intelligence headline moved to the HQ page** (the accessibility label still speaks the current state). **The phone radar (real and Demo)**
both use the disc — Demo shows new users what the real radar looks like and must match the device (Demo once kept `HQCard`, so when the app switched to the disc
Demo fell behind and users would take it for a second UI). **Only the iPad sidebar (`RadarPanel`'s sidebar form) still uses `HQCard`** (a floating disc does not suit a permanent sidebar).

**The disc's state model (`HQDisc`'s `discState`, highest priority wins; iron rule colour = status)** — red = needs attention (a decision or
a **real resource bottleneck**), the badge telling which:

| State | Condition | Ring | Centre/badge | Tap |
|---|---|---|---|---|
| Not started | no HQ session (owner only, and connected) | grey (whole disc dimmed to 62%) | `?` badge | explains what HQ is + how to start it on the Mac (the phone is remote, it cannot spawn) |
| Your call | HQ itself `waiting` | red | `!` | opens the HQ page |
| Someone is waiting | ≥1 worker `waiting` | red | count | opens the HQ page |
| Resource bottleneck | the machine at the **red tier**, a real bottleneck (`/api/usage` `resource.machine.tier==='red'`: disk critical / memory critical / load saturated; radar slow poll 25s) | red | `⚠` | opens the HQ page |
| HQ running | HQ itself `working` | cyan | — | opens the HQ page |
| All clear | otherwise | green | — | opens the HQ page |

Priority: your call > someone is waiting > resource bottleneck > running > all clear. **Not started** renders even without an HQ (grey disc +
explanation), but only once `conn==='live'`, to avoid flashing "not started" while connecting.

**Only the red tier turns the disc red (the low-noise iron rule)**: a soft **amber** hint (say 37GB free — below the 50GB amber line but far from empty,
or memory `warn`, load 1.0~1.5× cores) does **not** turn it red. The reason: red = "your call / act now"; a background disk hint painted the same red as
"a session is waiting for your input" makes the user think HQ needs them, and they tap in to find nothing (this was hit for real). Amber lives only on the HQ page /
the usage view, never on the "one glance" disc. `resource.machine.tier` is produced by Go's `WarnTier` (`amber`/`red`, omitted when normal),
and `disk_use_pct` samples the **data volume** `/System/Volumes/Data` (not the read-only `/`, whose capacity % is badly low and misleading).

**One recognition token across screens** (menubar-hq-state-parity): the menu bar's HQ card avatar is now **the same circular HQ badge**
(brand + "HQ" + status ring), with exactly the six-state priority of this `discState` (Swift side `AgentStore.hqState`,
resource likewise red only at the red tier, soft amber not red). The only difference is the container: the phone is a **draggable floating disc**, the menu bar a **card avatar**
(a popover is a fixed panel, nothing floats); and the menu bar's absent state can shell `gtmux hq` directly (the phone is remote and can only explain). When changing
the state model/colours here, sync the menu bar's `HQMedallion` and DESIGN §12.

Three screens (menu bar/phone/web), one mental model: verdict + your call + conversation, scaling with the screen. Implementation in `HQDisc.tsx` /
`HQScreen.tsx` + the pure logic `hqZones.ts`.


---

## Implementation alignment notes (2026-07 · round F)
See ITERATIONS-2026-06.md §F. Key points: billing moved entirely off the phone (the only purchase point = the Direct redemption code on the Mac); Servers grouped in two tracks (MY MACS / guest connections); Composer resting key row ⌨|Tab ↑ ↓ ⏎ ⌫ Ctrl-C Esc|quick replies▾ history (the user-facing copy has been 「常用语 / Quick replies」 since 2026-08; the internal code name stays snippets), the hard-coded 1/2/3 removed, replies belong to ApprovalCard (/api/options 1..N); Enter = newline, ↑ sends, ⤢ full-screen compose, attachments staged and uploaded on send; notification quick replies = three fixed digit keys without Enter; Settings = Moshi grouping + PickerSheet, owner items hidden from guests; iPad = the same app's regular shell (since 2026-09-12, §5); the HQ entrance in the phone radar = the draggable floating disc (`HQDisc`, logo + "HQ" mark, a 6-state status ring: not started grey / your call red ! / someone waiting red count / resource bottleneck red ⚠ / running cyan / all clear green; tapping when not started explains how to start it; the intelligence headline moved to the HQ page; Demo matches the real radar with `HQDisc`, only the iPad sidebar keeps `HQCard`, see hq-meta-layer).

- **Demo mode** (mockup §18): a full-function demo with no server (the App Review path). Iron rules: visibly sample data (the DEMO chip throughout), never mixed with real data (Servers has no entries), reset on every entry, every step guiding to "Pair your Mac". **Demo renders the real shells** (since 2026-09-12, SURFACES.md §3): `SplitShell` on iPad, `RadarPanel` on the phone; exit / sample banner / pair button are the panel's `demo` prop; it is no longer allowed a radar of its own (falling behind the device twice was caused by having one). The script's main line = the 30-second core loop: see one waiting → tap in → press 1 to approve → tests finish → the radar turns green with latest. Refinements in ITERATIONS §F7.
  - **Demo must be the same as the device radar, not a simplified version** (settled 2026-08-12, after two failures for this very reason):
    Demo is the **only** version of this app a reviewer sees, and a new user's first impression. Every **product capability** the device radar has,
    Demo must have — the 2026-08-12 audit found three missing: **the fleet count row**, **the "waiting only" filter**,
    **the "all panes" browser** (`demoClient` did not even have `panes()`; the whole surface did not exist on the review path),
    and "all panes" is precisely a capability the store description names. Also, tapping an ordinary pane fell into a "(no live screen)" dead end,
    now given a credible screen.
  - **Demo-only** things stay: the sample banner, the "Pair your Mac" button, the close button — demo scaffolding, not product differences.
  - **Chrome drawn on both sides is extracted into one shared component**, never written twice. §17 already recorded Demo falling behind the disc redesign because it kept its own
    `HQCard`; this time's count row/filter is the same mechanism. `ui/RadarSummary` is now shared by device and
    Demo, and `demoPanes()` is **derived** from `sampleAgents()` rather than a second table — derived things do not drift.


## 18. Server mode (display only, no control)

When the Mac is in server mode (lid closed, not sleeping), the phone must show it **at a glance** but **offer no switch**.

- **Presentation = a thin ring around the connection dot**: the same colour as the dot (connected green / reconnecting amber / offline red),
  shown only while server mode is on. Same colour by design — it reads as **another state of the same indicator**,
  not another thing to decode. The VoiceOver label appends "server mode on".
- **No chip, no list row, not in the radar**: server mode is a **machine** state, not an agent state.
- **The Servers page (my Macs)**: the currently connected Mac, if in server mode, gets **the same ring** around its connection dot
  and "server mode ·" in the subtitle. **Only the connected one is marked** — a Mac not connected cannot be asked, and inventing a state for it
  is worse than showing none.
- **The "Share & devices" page gets one read-only status row** (shown only while on): how long it has been on · power/battery (stating "sleep resumes automatically at 20%")
  · alerts for expiry or a missing daemon. **No buttons at all** — the ring in the radar is the "glance layer", this row the
  "sentence layer", and both only inform.

**Why there is no off switch** (settled 2026-07-31; an earlier version had one): **every management path of this feature ends in
"type an admin password once on the Mac"**. A remote switch that "can turn it off but not back on, and still makes you walk to the computer"
confuses more than none. The capability stays in the API and the client (that de-privileging can be initiated anywhere is a security invariant); it is just not made into UI.

**States and edges**: no server mode → no ring · guest token → even reads get 403, entirely invisible ·
remote enable → the server answers 403 to every client (must be authorised on the Mac).
