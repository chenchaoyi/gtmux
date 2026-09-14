# gtmux menu-bar app — Design Requirements

> This file is the **authoritative design spec** for the gtmux menu-bar app. The implementation follows it;
> for visual reference see `docs/design/mockup/` (interactive prototypes) and `docs/design/mockup/preview-*.png`
> (static screenshots). Every UI change must respect the tokens, status language and interaction rules defined here.

Target implementation: **native macOS — NSStatusItem + NSPopover + SwiftUI** (a custom popover view,
not a system NSMenu). Data comes from `gtmux agents --json` (contract in `internal/menubar/model.go`).

---

## 0. Design principles

1. **A glance tool**: looked at dozens of times a day. Quiet, fast and scannable beats decorated.
2. **Hierarchy is everything**: "needs you" must ring; "idle" must be quiet enough to almost not register.
3. **Never colour alone**: every status = colour + shape + glyph, triple-encoded (readable colour-blind, in peripheral vision, and on a tinted menu bar).
4. **Native, restrained, trustworthy**: this is a developer tool, not a consumer app. No rainbow gradients, no glowing shadows, no marketing copy.
5. **Minimal motion**: at most one cue on idle→waiting; zero animation otherwise.

---

## 1. Status model (core)

Every agent has four statuses (matching the CLI, see `internal/app/agents.go`):

| status | meaning | colour (authoritative, from `internal/menubar/icon.go`) | shape | glyph (white, inside the badge) |
| --- | --- | --- | --- | --- |
| `waiting` | blocked on you, waiting for your input (most urgent, sorted first) | `#EF4444` red | **square** (corner radius ~3.5px) | **double bar ⏸** (pause) |
| `working` | busy, running (don't interrupt) | `#06B6D4` cyan | circle | **loading ring** (open ring, **slowly rotating**, see §10) |
| `idle` | this turn finished, your move (not urgent) | `#22C55E` green | circle | **checkmark ✓** (done) |
| `running` | alive, sitting at the prompt | `#8E8E93` grey | circle | **small dot** |

- "Most urgent status" priority: `waiting > working > idle/running`.
- This glyph language is **uniform across every surface**: the menu-bar status item, popover row badges, the legend and Preferences all use the same set.
- The colour channel is reserved **only** for status; never use it to tell agent types apart (see §6).

---

## 2. The menu-bar status item (NSStatusItem) — the most important surface

A 16pt glyph that must be readable in a second in peripheral vision, colour-blind, and on light / dark / tinted menu bars. **WAITING must be carried by shape + glyph, not colour alone.**

### Glyph (recommended: Shape-shift, the shape changes with status)

The status item's glyph changes shape with the "most urgent status", not just colour:

- **calm (0 / all idle, optionally hidden)**: a grey **hollow ring**.
- **idle**: a green **✓** (or sparkle).
- **working**: a cyan **loading ring** + count. **This one instance does not rotate** (§10): it sits on screen all day, and motion there is noise; what rotates is the inline badge in the popover / phone / web.
- **waiting**: a red **square + double bar** + count.

The shape itself is the signal → readable in peripheral vision and colour-blind.

### Three display modes (a preference, matching §10 "Status bar display")

1. `dot` — dot only (colour = most urgent status).
2. `dot + count` (default) — dot/glyph + the count of the most urgent actionable status (`waiting`, else `working`, see
   `BadgeText`).
3. `hide-when-idle` — when nobody is waiting on you, the status item is **hidden entirely**; never intrudes.

### Rendering and adaptation

- Coloured glyphs are **non-template images**, so red/cyan/green survive when the system tints the menu bar
  (current state: `IconFor`/`dotPNG` in `icon.go`, non-template).
- **The count digits are a template image**, following the menu bar's black/white automatically.
- **Tinted wallpaper**: when the red square has too little contrast on a red/orange wallpaper, add a **0.5pt white outline** automatically (or flip to a white fill).
- Ship **SF Symbols + tint tokens** first, falling back to **@1x/@2x PNG** (22×22, anti-aliased, per `dotPNG`).
- Laptop **notch**: the status item sits right of the notch, so its width budget is small.

### tint / symbol tokens

| status | SF Symbol (suggested) | tint |
| --- | --- | --- |
| waiting | `pause.fill` / custom double-bar square | `#EF4444` |
| working | `circle.dotted` / loading ring | `#06B6D4` |
| idle | `checkmark` / `sparkle` | `#22C55E` |
| calm | `circle` (hollow) | `#8E8E93` |
| count badge | template text | auto B/W |

---

## 3. Popover (NSPopover + a custom SwiftUI view)

Opened by clicking the status item **or** by the global hotkey.

### Dimensions (pt)

| item | value |
| --- | --- |
| popover width | **420** |
| corner radius popover / row / chip | 13 / 8 / 5 |
| row height (two-line, default) / compact | 46 / 28 |
| agent avatar | 30 |
| list max-height | 360 (scrolls past 7 rows) |
| padding / inline gap | 7 / 11 |

Material: vibrancy (frosted glass). Light and dark both supported.

### Structure (top to bottom)

1. **Header**: the gtmux mark (pane grid, see §15) + a "waiting only" filter toggle + a search button; the next line is
   the summary `5 agents · 1 waiting · 2 working · 2 idle` (localised, see `Summary`). In search mode the summary
   line becomes the search field.
2. **Grouped list**: the section order is fixed, **needs you (waiting) → errored → working →
   idle → standby (running)**. Each section has a small title (all caps, weight 700, +0.5 tracking) + a count;
   **the waiting section title is red**, **the errored section amber**, the rest neutral.
   - **"Errored" is a section, not a status** (2026-08-17): the status language still has exactly four entries; errored describes
     **how** this turn's idle **ended**. It gets its own section because "finished" and "stopped on an error" ask different
     things of the reader; mixing the latter into the former leaves a session that needs a human lying in "done" under a
     green ✓ (the commander hit this in practice).
   - **Never fold it into "needs you"**: that section means "the agent is asking you a question you can answer", and an error
     is not a question; you can't answer it, you can only go fix the environment. Folding it in dilutes the iron rule
     "red = waiting on your input".
   - Same for the summary line: errored is **counted separately**, no longer inside idle (otherwise the summary disagrees
     with the sections below it), and appears only when non-zero. Its collapsed state is independent of idle too; collapsing
     "done" must not hide the failures with it.
   - **Every form factor matches**: menu bar, phone, iPad and web all use this order and these section colours (the web caught up on 2026-08-17, iPad on 2026-09-12; the inventory is in `SURFACES.md`).
3. **Footer**: four action buttons **Overview · Live watch · Restore · New session** (icon + text); a divider;
   a gear **Preferences** on the left, the version line `gtmux 0.1.0 · designed by ccy` on the right.

### Row model

The row starts with the **agent avatar** (identifies "which tool"), with a **status badge** (§1's colour + shape + glyph) overlaid at its bottom-right.

```
[ agent 头像 30pt + 右下角状态徽章 15pt ]  session(主) · window(次)  [latest?]
                                            task（次要、灰、省略号截断）        time   ›/⏎
```

(agent avatar 30pt + status badge 15pt bottom-right · session (primary) · window (secondary) · optional latest · task (secondary, grey, ellipsised) · time · ›/⏎)

- **First line**: `session` (bold primary, 13/590) + `window` (secondary, grey, 11.5) + an optional `latest` marker.
- **Second line**: `task` (12, grey, `nowrap` + ellipsis; `—` when empty).
- **Right column**: relative time (mono, tabular-nums) + a jump marker (`›`→`⏎` on hover/selection).
- **Waiting rows get extra weight**: a faint red background `rgba(239,68,68,0.08)` + the red section title + the red square badge (pulsed once if needed).
  Every other status stays quiet (neutral avatar, low-saturation badge).
- `latest` = the agent that finished most recently, marked with a green text pill "latest / 最近完成" (**do not** add another ✓;
  it would duplicate the idle badge's ✓).

### Interaction states

- **hover = selection**: moving the mouse onto a row moves the keyboard selection highlight there (a single highlight, command-palette style).
- Selected row: `rgba(255,255,255,0.12)` (dark) / `rgba(0,0,0,0.07)` (light), the right marker becomes `⏎`.
- **Keyboard**: `↑/↓` move, `⏎` jumps (`gtmux focus <target>`), `⎋` leaves search / closes.
- Clicking a selected row or `⏎` → calls `gtmux focus` (pane id for tmux; native, see §7).
- Scrolls past max-height; section titles may be sticky.
- The "waiting only" toggle filters the list down to waiting.

---

## 4. Quick switcher (global hotkey, default ⌥⇧G)

Two forms; build one or both:

- **A · popover search mode**: typing inside the popover fuzzy-filters the same list (reusing every status and row style). In place, zero navigation.
- **B · standalone command palette** (recommended as the hotkey entry): a centred, Raycast-style panel, large hit areas, keyboard-first, a shortcut bar at the bottom
  (`↑↓ 选择 · ⏎ 跳转 · ⌘1–9 直达`, i.e. ↑↓ select · ⏎ jump · ⌘1–9 direct).

Fuzzy-matched fields: `session / project / window / task / agent / pane`.

---

## 5. Empty state & first run

### Empty state

No error, no awkward blank. Show one copyable launch command; the copy says **any coding agent** (not just Claude):

> 没有运行中的 agent
> 在 tmux pane 里启动任意 coding agent（Claude Code · Codex · Gemini · Cursor…）
> `tmux new -s work \; claude`

(No agent running · Start any coding agent in a tmux pane (Claude Code · Codex · Gemini · Cursor…) · `tmux new -s work \; claude`)

### First run (Automation permission)

`focus` uses AppleScript to switch the terminal to the right tab/pane, which needs a one-time macOS "Automation" permission. Use a friendly
explanation card rather than throwing the system dialog straight at the user.

**Copy requirement: plain and matter-of-fact, no marketing voice** (nothing like "see who's waiting at a glance" or "one click to jump"). Example:

> **跳转需要「自动化」权限**
> 点击某个 agent 时，gtmux 用 AppleScript 把它所在的终端标签页和 tmux pane 切到最前。这需要一次
> 「自动化」授权，只切换窗口、不读取终端内容。
>
> 1. 点「允许并继续」，会弹出 macOS 系统对话框
> 2. 在「"gtmux" 想要控制 "Ghostty"」中点「好」
> 3. 随时可在 系统设置 › 隐私与安全性 › 自动化 撤销
>
> *不授权也能用：`agents`、`overview` 照常工作，只是不能点击跳转。*

(Jumping needs the "Automation" permission. When you click an agent, gtmux uses AppleScript to bring its terminal tab and tmux pane to the front; that needs a one-time Automation grant, which only switches windows and never reads terminal contents. 1. Click "Allow and continue"; the macOS dialog appears. 2. In "gtmux wants to control Ghostty", click OK. 3. Revoke any time under System Settings › Privacy & Security › Automation. Works without it too: `agents` and `overview` keep working, you just can't click to jump.)

---

## 6. Agent identity (tell them apart, but don't colour them)

- The row avatar defaults to a **neutral monogram** (`C` / `Cx` / `G` / `A` / `oc` / `Cr` / `Cu` / `Am`), **monochrome and neutral**,
  never competing with the status colour.
- **No brand colours for agents** (purple/orange would fight the status colours and dilute waiting's red).
- **Real logos are third-party trademarks and are not drawn in the design.** The system leaves a hook: add an
  `icon` field to the agent profile (`~/.config/gtmux/agents.json`, see `agentProfile` in `internal/app/agents.go`), and the app
  loads the **official icon** at runtime under each vendor's brand rules (the repo hook already has precedent, caching the Claude icon).
- A Preferences toggle "show agent name": when on, the second line is prefixed with a dim agent name (`Codex · task…`).

---

## 7. tmux and native terminals (generalising the data model)

gtmux supports not only agents inside tmux but also agents **running directly in a native terminal** (no tmux).

| source | primary id | secondary id | jump target |
| --- | --- | --- | --- |
| `tmux` | `session` | `window` | `gtmux focus <pane_id>` (`%N`) |
| `native` | `project` (cwd basename) | `terminal` (Ghostty / iTerm2 / Warp / Terminal…) | focus the tab titled `tab` in the `terminal` app (AppleScript) |

- A native-terminal agent **has no tmux session/window/pane**; the row's primary id becomes `project`, the secondary `terminal`,
  plus a small `native` marker.
- The `agents --json` contract **gains these fields this round**: `source: "tmux" | "native"`, `project`, `terminal`,
  `tab` (the native tab title) (tmux entries may omit project/terminal/tab; native entries may omit session/window/pane/loc).
- **The native jump target = "terminal app + tab title"** (`terminal` + `tab`): AppleScript selects the tab whose title matches
  `tab` in that terminal and `activate`s it. (Decided.)

---

## 8. Preferences window

The standard macOS settings grid (labels right-aligned, controls left-aligned). Fields:

- **Language**: `跟随系统` (follow system, default) / `English` / `中文`. Reads `GTMUX_LANG`; once locked it is written to preferences and overrides.
  Switching takes effect **immediately**: status item, popover and notifications all follow.
- **Refresh interval**: slider, default 1.5s.
- **Launch at login**: toggle.
- **Status bar display**: `点+数字` / `仅圆点` / `空闲时隐藏` (dot + number / dot only / hide when idle; the three modes of §2).
- **Global hotkey**: recordable (default ⌥⇧G).
- **Notifications**: toggle (alert when an agent starts waiting on you / finishes).

---

## 9. Design tokens

### Colours

```
# 状态色（权威，icon.go）
waiting  #EF4444   working  #06B6D4   idle  #22C55E   none/running  #8E8E93

# 深色 popover（vibrancy）
bg      rgba(28,28,31,0.60) + blur(28px) saturate(180%)
fg/2/3  rgba(255,255,255,0.95) / rgba(235,235,245,0.62) / rgba(235,235,245,0.34)
divider rgba(255,255,255,0.09)    row-selected rgba(255,255,255,0.12)

# 浅色 popover
bg      rgba(252,252,253,0.72) + blur
fg/2/3  #1d1d1f / rgba(60,60,67,0.62) / rgba(60,60,67,0.34)
divider rgba(0,0,0,0.08)          row-selected rgba(0,0,0,0.07)
```

(Status colours, authoritative in icon.go · dark popover (vibrancy) · light popover.)

> Backgrounds / wallpapers use **neutral, low-chroma** colours; rainbow gradients are forbidden; cards get no coloured glow shadows.

### Typography

- UI: the system font `-apple-system` / **SF Pro Text/Display**; Chinese in **PingFang SC**.
- Code / pane id / loc / commands: **SF Mono** (`ui-monospace`).
- Sizes/weights: command-palette title 18/680 · session 13/590 · task 12/400 · section title 11/700 +0.5 caps · mono 11.

### Spacing / radii

8pt grid. See the §3 dimension table.

---

### A row you can't jump to has to say so first (2026-08-16)

Clicking a row means "take me there". But a session with **no terminal client attached** (dispatched with `--headless`, or
detached by hand) has no tab to jump to at all; the old code did its select inside tmux, then went looking for a terminal tab
that could not exist, and **nothing happened, silently**. All the user can conclude is "click-to-jump is broken".

- **The criterion is the client count (`session_attached`), not "how it was started".** Measured: the `restart-feed` window
  carries the `⌁` headless marker in its name, yet attached=1 and it jumps fine; `disk-drift-triage` has attached=0 and can't.
  The marker records where a session came from; the attachment state says whether there is a window right now.
- **Behaviour**: when nobody is attached, `gtmux focus` **opens a tab and attaches** (the same call `gtmux new`/`restore` use).
  All three screens share this one primitive, so focus from the phone does the same: back at the desk, it is already open.
- **Presentation**: such rows get a small neutral tag `no window / 无窗口` (**not amber**: it is not a fault, it sits among a
  crowd of ordinary idle rows), with a tooltip saying "clicking will open a window for it".
- **Desktop notification subtitles also start with `%N`** (2026-08-17). A notification is the **only surface with no list to
  look at**: one title, one subtitle the system truncates, nothing else. Measured on a fleet of 4 sessions each holding two
  panes, the worst pair was "title MP / subtitle multipilot-companion 服务端需求" versus "title MP / subtitle multipilot-companion
  featu…": same session name, same beginning, the rest cut off, two banners impossible to tell apart. Four characters of id
  solve it, and that id is exactly the one the click jumps to, the one the pane browser leads with, the one HQ names.
- **`%N` may never be squeezed out**: the second line is normally `会话 · %N` (session · %N), but the error / background-run
  labels used to **replace the whole line**, so the row that most needed identifying was the only one without a pane id. The id
  now sits first on the second line, ahead of the long copy that gets truncated.

### A slow action must speak for itself, and needs a concurrency floor (2026-08-17)

The "restore your last workspace" row got three things wrong at once, each causing the next:

- **Too light**: at 11pt / fg2 / no background it was the weakest element on the whole panel, and the reason you are looking at
  the panel is that there is nothing above it. "We haven't counted the sessions yet" is a reason to word it carefully, not a
  reason to whisper **the only action on screen**. It now carries the same weight as the "N sessions known" version.
- **Nothing changed on click**: restoring opens a pile of tabs and waits on the terminal for each, which can take several
  seconds, and the row looked identical before and after the click. What the screen honestly told the user was "nothing
  happened", so **clicking again** was an entirely reasonable response. Now the click swaps the whole row for
  "正在恢复工作现场…" (restoring…) + a spinner, and it is **not clickable**: the honest form of "clicking again does nothing",
  rather than a button that quietly swallows the click.
- **No floor**: a repeated trigger **restored the entire workspace twice**. A disabled control in the UI cannot stop a second
  entry point (`gtmux restore` in a terminal, the phone, or the click that got in first), so the real backstop is in the **CLI**:
  a run lock carrying pid and start time, one restore at a time; a dead process or an exceeded limit is taken over automatically
  (a crash must not jam the feature forever); `--plan`/`--dry-run` are exempt, because refusing to **show** the plan during a
  restore is the guard standing between the user and the answer.

**The general rule**: any action that takes seconds and changes the world needs both a visible "in progress" state **and** a
concurrency guard that does not depend on the UI. Only the former, and a second entry point still double-runs it; only the
latter, and the user still clicks repeatedly for lack of feedback.

## 10. Motion

- **The working loading ring rotates slowly** (revised 2026-08-13; it was specified static before). Reason: of the four
  statuses, **only working is a "process" rather than a "state"**; the other three are conditions at a moment, working is
  "happening now". A static ring makes the shape do the decoding; a rotating one says it outright. **One turn per 2 seconds,
  linear**, which reads as "alive" rather than "urgent": urgency is red's job, and **red never moves**.
  - **It rotates only in an interface the user has open**: the popover, the phone app, the web page. **The menu-bar status
    item icon stays static**: it is on screen all day, and motion there is ambient noise, not information. (The server-mode
    breathing dot is a separate, pre-existing exception, and it likewise exists only while enabled.)
  - **Respect the system "reduce motion"** (macOS `accessibilityReduceMotion` / iOS `AccessibilityInfo` /
    web `prefers-reduced-motion`). A glance tool has no standing to override that choice.
  - All three surfaces share one set of parameters, because they are one recognition token: the ring you see on the phone
    and the web is the same ring as in the menu bar.
  - **"Only rotates while open" must be guaranteed by code, not just written in a comment** (measured 2026-08-15).
    Once `.repeatForever` starts it runs until the view goes away, and the menu-bar app's two interfaces **keep their views
    after closing**: the popover's `NSHostingController` is held long-term by `NSPopover`, and the "all panes" window is
    `isReleasedWhenClosed = false`. So the ring kept spinning in windows you could not see, stacking one more layer per open.
    Measured (same build, same fleet, one working agent): **0.1–0.3% CPU before opening, 4.6–11% after open-and-close and
    still climbing**; the commander's menu bar after 3.5 hours sat at 20%, 87.9% with the interface open.
    **"Make the ring stop" is a dead end**: with a visibility gate added and the state genuinely flipping to false, CPU did
    not move; a running repeatForever does not stop because its driving state changed or its animation was swapped for nil.
    **The only thing that ends the animation is "this view does not exist".**
    But **don't tear down the whole view tree** (tried 2026-08-16, wrong): CPU did drop to 0.0–0.2%, but every reopen laid
    out from scratch; measured, the first frame was `root=168` while the window still held last time's 983, so a blank band
    at the top of the panel and the last rows falling off the bottom. **Removing the one smallest view that carries the
    animation is enough**: the ring is split into a "rotating" and a "static" view, and the static one is swapped in when the
    interface is not visible; no view, no animation, while the rest of the panel stays alive and measured. Measured 1.0–1.3%
    and no longer climbing, the view tree built once.
  - **The panel's size can have only one authority** (measured 2026-08-16). There were two: SwiftUI measured the panel height
    and pushed it into `popover.contentSize`, while the hosting view was sizing itself too, and the two disagreed:
    `asked=420x983` versus `drew=420x849`, 134pt apart, which NSPopover filled with its own background, so a blank band at
    the top of the panel and the first list row pushed past the bottom edge. (849 = chrome + the list's **first** measurement
    682; 983 = chrome + the settled 816; the window followed the latter, the view stopped at the former, and nobody corrected
    anybody.) Now `NSHostingController.sizingOptions = [.preferredContentSize]` alone owns the size; the SwiftUI side keeps
    only the **clamp to screen height** (`listMaxHeight`), and that sink is left with just the positioning job of "move the
    window back on screen after the panel grows". A debug build prints a MISMATCH line when asked and drew differ by ≥2pt;
    on screen this inconsistency shows only as a blank panel, which is exactly why it went unnoticed for two releases.
  - **Live Activity (lock screen / Dynamic Island) cannot rotate; a platform limit, not our choice.** Tested on a real device
    on 2026-08-13 (iPhone 15 Pro Max / iOS 26.6): a `.repeatForever` rotation sent straight into the widget **does not move**.
    WidgetKit renders timeline snapshots; between updates only system-driven views update themselves.
    **But the lock screen is not missing the signal**: the elapsed time at the right of each row is `Text(style: .relative)`,
    which the system ticks every second. The same sentence (happening now, for how long) is said by whatever mechanism each
    surface **actually has**.
    **Do not fake rotation with push**: Live Activity updates are rate-limited, and burning push budget and battery to spin a
    ring is a bad deal on a lock screen.
- **The one other permitted motion**: on the idle→waiting transition the status item / badge gets **one** pulse cue.
- Idle is **zero animation**. Hover/selection/open may use very restrained micro-transitions; the whole stays quiet.
- **Still forbidden**: page-level spinning loaders (`BrandLoader` therefore pulses instead of spinning; it means "the page is
  loading", not "an agent is working", and the two must not share a motion language).

---

## 11. Accessibility (VoiceOver) & i18n

- Each row = one button. label = "session, agent, status, task, time"; hint = "jump to this pane / terminal".
- **Never** let colour carry meaning alone (shape + glyph already provide the redundancy).
- Hit area ≥ 44pt (menu items may be slightly smaller, but the clickable area must be enough).
- **i18n**: `GTMUX_LANG` en/zh; the language preference has three states. Chinese (CJK) is wider → fixed row height, a flexible middle column, `nowrap` + ellipsis,
  **never wraps or overflows**. Counts are digits, so width stays stable.
- **Three tiers of English casing, one rule across all three surfaces** (decided 2026-09-05; the authoritative version is MOBILE §6):
  ① **Sentence case** for names and actions (tab/segment titles, buttons, sheet titles, settings rows):
  `Situation board` / `Knowledge` / `Mark it landed…` / `Focus`; capitalise only the first word, proper nouns keep theirs
  (`HQ` / `gtmux`). ② **All lowercase** for the status words themselves (`waiting` / `idle` / `errored` /
  `read-only`); they are the status language shared by all three surfaces, and **a status is not a name**. ③ **Small all-caps** for the key
  next to a value (`WAITING ON YOU` / `NEWEST`), **produced by `.uppercased()` from lowercase source**, so it is always a
  visual tier and never a spelling choice. Verbs in the keyboard hint bar (`↑↓ select`) are captions and ① does not apply.

---

## 12. Logo / brand mark

- **App mark = the pane grid (option C)**: a 2×2 grid with one cell highlighted **`#06B6D4` cyan**, the rest neutral, on a dark square-cornered icon.
  Used in the popover header, the empty state, first run, the quick switcher, Dock/About.
- The three dots (red/cyan/green) are demoted to a **secondary motif** (legend, emphasis); no longer the main logo.
- **Status item ≠ logo**: the menu bar shows the **status glyph** (§2), not the app logo.

---

## 13. Status and edge-case matrix

| scenario | status item | popover |
| --- | --- | --- |
| 0 agents | grey hollow ring / may hide | empty-state card + launch command; no error |
| 1 waiting | red square + count + one pulse | the single row lands straight in "needs you", pre-selected, ⏎ jumps |
| ~5 mixed | the most urgent wins (red first) | three sections; waiting highlighted, idle quiet; latest marked |
| 15+ | shows only the to-do count, never blows the bar width | scrolls after 360pt; "waiting only" narrows it |
| very long task | unaffected | single-line ellipsis, tooltip carries the full text; session is never squeezed |
| CJK Chinese | count is a digit, stable width | fixed row height + flexible middle column + ellipsis, no wrap/overflow |
| native terminal | as above | primary id is project, secondary terminal, with a `native` marker |
| idle→waiting | green circle → red square + one pulse | the only moment of motion |

---

## 14. Footer v3 (layered by frequency) · §14 mockup

The permanent footer is down to **one row**: left "＋ New session", right the ⚙︎ menu (Preferences… ⌘, / Pair a device… / Check for updates · version / Quit ⌘Q). The **contextual notice row** "↩ N sessions to reattach" appears only when detached sessions exist, and clicking it restores. Connection/sharing state (green dot + device count, the "input" chip) is inline in the permanent row and shown only when true. Buttons are icon + text on one line throughout (no more two stacked lines); the version number moves into the ⚙︎ menu; pairing also has a CTA on the empty-state card.

**The restore row has three states (decided 2026-08-11, because "nothing is running" and "something is waiting to come back" are two different things that used to look the same)**:
① **Sessions can be restored** → the **bar** form: filled background (`rowSelected`), `fg` semibold, chevron; the same
"gtmux is handing you a one-click action" shape as the update bar. ② **Plan fetched and empty** (fresh install) → **the row does not
appear at all**; the empty-state card's tmux guidance is the only thing that belongs there. ③ **Plan unknown** (not yet polled / the CLI
call failed) → keep the old quiet row; not knowing is not the same as none, and quietly removing a usable entry point because one
shell-out failed is the worse mistake.
**The update bar's cyan is deliberately not reused**: the two render **vertically adjacent** in the footer, and two identical
prominent bars pointing at different actions is exactly how mis-clicks happen; besides, cyan means `working` in the status language.

**The restore row expands (restore-plan)**: the contextual row has a chevron on the right; expanded, it lists **what will be restored**: each session (name + `Nw·Mp` window/pane counts) and, under it, the agent sessions that will be reattached (↻ green = reattachable / × grey = transcript lost, will not be restored; the label is the session goal, else cwd). The main label still restores in one click; the chevron only expands. Data source = `gtmux restore --plan --json` (read-only, fetched only when the restore row is shown, i.e. no session running). **Purpose**: after a reboot, review first, then decide, and match the very same plan `gtmux restore` prints in the CLI (one data source, two screens). The chevron is hidden when the plan is empty.

## 16. Pane browser · tiered-pane-control (**separate from the radar, a red line**)

gtmux's control primitives (`focus`/`send`/`attach`) were always **pane-level** and work on any tmux pane; only the radar's "smart" layer (detection/digest/1·2·3/dispatch/HQ) is agent-only. So "manage every pane" is a **presentation decision**, not a capability refactor. **The one red line: never flatten every pane into the radar**, or vim/htop/logs crowd in with the agents, "who is waiting on you" gets diluted, and the radar degrades into yet another tmux session list (tmux itself is that red ocean).

Three surfaces, tiered and separated:

- **A standalone browser window** (`⚙︎ → 浏览所有 pane…`, "browse all panes…", **not part of the radar popover**): the session→window→pane tree from `gtmux panes --json`, each row tagged `tier=agent|plain`. Agent rows get a ▸ marker + name/title; plain rows = the command name. Clicking any row = focus; hovering a plain row shows a 👁 watch toggle (pins it onto the radar). This is where "all of tmux" is managed, which is what keeps the radar clean. Panes of one session naturally cluster together → the desktop side's "agent neighbour panes" are covered here (the phone Detail's neighbour strip is in MOBILE).
- **A watched section inside the radar** (below §3): opted-in plain panes appear as their **own section**: a thin divider + a 👁 "watched" small title, rows carry 👁 (**no agent status**: waiting/working/idle are agent concepts), placed after every agent, dropped automatically when the pane closes. **Never added automatically.**
- **The tier contract**: agent = the full smart set; plain tmux pane = focus/input/watch/attach; non-tmux agent = read-only (Elsewhere). One `send`/`attach` for all; the only difference is whether the radar gives it the extra buffs. Guest scope applies to any pane as usual.
- **The browser window aligns with the phone's version of this screen** (2026-08-12): session groups are **collapsible** (chevron + name, choice remembered),
  the header has **one-click collapse/expand all**, a **persistent search field** (session/command/directory/title/agent), and the count line reads
  `N 个 pane · M 个会话` (N panes · M sessions), with the "K waiting on you" part in waiting red. Session headers carry a **rollup** (pane count · agent
  count + a pip per status). **The rollup lives on the header so it still speaks when collapsed**: folded, you must still see "1 in here is waiting on
  you", or collapsing hides exactly the reason this screen exists. Agent rows show the **real status badge** (joined to the radar by pane_id).
  Label rules share a source with the phone (`PaneLabels` ↔ `api/types.paneLabel`): an agent row never takes `command` as first choice
  (Claude 2.x's `pane_current_command` is a version number, #659). **Plain rows need a name, not a command name**:
  printing only the command makes every shell on a machine `bash`, true but **distinguishing nothing**, and these rows are exactly where the reader picks one.
  Naming order follows "what this step says beyond the previous one": `title` (someone chose it deliberately; **a whole path is not a name**, many shells
  write cwd into the title, sometimes with a colon prefix) → `win_name` (unless tmux's automatic-rename turned it into the command name) →
  `project` (which repo, stable across subdirectories, and what people actually call it) → the last segment of `cwd` → `command` (the last resort).
  Each screen has its own implementation (different languages), but **the same chain and the same test cases**; changing one side means changing the other.
  Two separately written rule sets is exactly how they diverged in the first place.
- **Identity uses tmux's stable ids (tmux-id-surface, 2026-08-14)**: what a row is recognised by is the foundation of this screen.
  The rule in one line: **the id is the anchor, the name is annotation, the sigil marks the tier**: a session uses its **name** (unique already,
  and what `attach -t` accepts); a window uses `@id + name` (tmux's `automatic-rename` makes names drift, and two windows can share a name, so
  `@id` is the anchor); a pane uses `%id + gtmux's own derived label` (**not** `pane_title`; measured across a whole fleet, every plain shell pane's
  `pane_title` was the same hostname).
  On each form factor:
  - **`%N` leads the row**, not trailing on the right. It is the row's only stable and unique token (the tab title, `gtmux focus %N`, and what HQ says
    all use it); the old `w.p` coordinate is **deleted**: it changes whenever panes are rearranged, and a coordinate that changes does not deserve a column.
  - **Every session draws its window bands** (`@id name`, faint background), **including single-window sessions**.
    This was reversed once on 2026-08-14: the original reason for "don't draw it for a single window" was "save a row", but that was counting rows,
    not looking at the tree. A single-window session then put its panes directly under the session header, and that indent elsewhere means "window",
    so **the same shape one row apart meant two things**, and you had to decide which kind of session it was before reading. One row for a predictable
    three-level tree is worth it. A collapsed session header must still list all its window ids (folded to `+N` past 5); folded, it must still say what is inside.
  - **The agent name is no longer repeated inline.** Six rows all saying "Claude Code" repeat what the official icon at the row start already said;
    the icon carries identity, the text is kept for "what it is doing". (The menu bar keeps the name in the tooltip, so it is still there when the icon
    can't be fetched and the monogram is the fallback.)
  - **`%N` is click-to-copy** `gtmux focus %N`, with **an inline "copied" echo** (a copy with no echo is indistinguishable from a click that did nothing);
    the click acts only on the id, the whole row is still "open this pane".
  - **Ids are searchable**: `%23` / `@17` / bare digits all match. A token visible on screen must be recognised by the search field.
  - Honest limit: tmux ids are stable only within one server's lifetime (`kill-server`/a reboot renumbers), so they are the anchor for
    "the fleet right now" and **must not** be used as a persistent key across restarts.
- **The browser window's height follows the session count** (2026-08-12): it used to open at 620pt whether the machine had 3 sessions or 81.
  Now it opens at the **measured content height** (chrome + the list's wanted height), minimum 400, maximum = usable screen height − title bar − 120
  (with real margin: a window that nearly fills the screen reads as "took over the screen" rather than "a panel you opened", and the list scrolls anyway).
  Width 480 is written as a **minimum width** rather than idealWidth: `NSHostingController` sizes the window by the minimum, and idealWidth measurably
  does nothing (the window came out at 480, measured 420). Same decomposition as the popover, but **a different container and different rules**: a window is
  resizable and belongs to the user, so this is "how big to open", not a cap enforced during use; once the user drags an edge (`didEndLiveResize`) it yields permanently.
  And **fitting once is not enough**: the pane list is fetched asynchronously, so the moment the window appears the content is a few points tall, and a
  one-shot fit locks in `wanted=8` (measured). So keep fitting for **2.5s after opening**, then stop; panes poll in and out every 1.5s, and a window that
  grows by itself under the reader's eyes is worse than one whose size is imperfect.

**Positioning red line**: gtmux remains a "coding agent command center"; "any pane reachable" is a **capability**, not the **headline**. The story must not become "a general tmux manager".

## 15. References (borrowed from)

Tailscale (a restrained status item, connection state readable at a glance) · OrbStack (two-line list, gentle material) ·
Stats/exelban (in-bar density, configurable "what to show") · Raycast (command palette, fuzzy search, bottom shortcut bar) ·
Dato/itsycal (lightweight native fit) · CCMenu (the mature status + list + jump pattern).


---

## 12. HQ (supervisor) · §12 mockup

`gtmux hq` = `role:"supervisor"`, the meta session that watches the whole fleet. The menu bar sets it apart with a **chief-of-staff card**:
- **Entry**: a "👁 CHIEF OF STAFF · 参谋长" role banner + a 1px outlined panel (no agent row has one), pinned between the summary and the list; clicking it = **jump to the HQ pane**. The command deck lives on the phone / web.
- **The two windows open from two small marks at the right end of the role banner** (`态势板` · `知识库`, situation board · knowledge base, `HQReader.swift`, each opening its own window). **Icons only, no text**; the names stay in the tooltip and accessibility label: they are entries, not controls, and the banner row exists anyway. Earlier they were a pair of full-width buttons under the card that spent the most expensive full row of the popover on two secondary entries (the line at the top of this section: the most expensive pixels cannot be sold to something one step away). When HQ isn't running, that row is still the original explanatory sentence. This is a **deliberate revision** of the early "clicking only jumps to the pane, no separate panel" rule; the boundary is redrawn between **reading / driving** rather than between **screens**:
  - What the original rule constrained was **driving** (dispatching, sending messages, deciding), which needs a composer and stays on the phone / web unchanged; these two windows **offer no driving action at all**.
  - **Reading** is exactly what should happen in front of the Mac: the situation board describes this very machine, and the reader is already sitting at it.
  - **The knowledge window may carry judgement-type actions** (menubar-kb-actions, replacing the early "land / retire stay out of the menu bar"): the four verbs `promote` / `land` / `retire` / `dismiss` are all judgements on **something already written**, and the cheapest place to judge is where you read it. After hq-knowledge-engine there are three more exit-type actions, still judgement, not authoring: `promote` first asks **who this one is for** (HQ / this machine / the repo / everyone, single choice); entries awaiting take-away get an exit per reader: HQ / this machine / the repo are "write it in" (gtmux moves it for you, `land` without a ref), everyone is "feed back to gtmux ↗" (opens a pre-filled issue, then `land` with the link); `withdraw` takes back a promotion not worth moving. The entry detail gains a three-axis line (kind · source×count · reader), and candidate rows carry a "≈ same thing" family number, matching one `add --capture k1,k2,…`. **Authoring actions still stay out of the menu bar**: `add` / `supersede` need a body typed in, and a body typed into a popover fills the knowledge base with entries nobody wants to read sooner or later; that is the very reason the phone's gate opens only two verbs. So what moved this time is the "read / write" line, replaced with "judge / author"; **driving the fleet is untouched**, the red line stays where it was.
  - **Which entry gets which action is decided by lifecycle, and the CLI is the authority**: promoted-but-not-landed entries (`待你带走`, awaiting take-away, still listed first) get `land`, other valid entries get `promote`, both get `retire`; the candidate queue (`gtmux capture`'s pending distill pool) gets `dismiss`, grouped by dedup key, because `dismiss --capture <key>` clears every row with that key. `promote` on an already promoted entry is refused by the CLI ("land first"), and a button whose only output is an error is simply not offered.
  - **Form and wording follow the phone** (`KnowledgeSheet.tsx`): an entry row only opens; actions hang inside the entry detail, judged with the body in view; **a reason is required** (the CLI already demands it: `promote/retire/dismiss` take `--why`, `land` takes `--ref`), **one confirmation before executing**, the confirmation naming verb and object; on failure **the CLI's stderr is shown verbatim**, unprocessed: "you need a pending promotion to land" already says what to do next, and replacing it with a home-grown "operation failed" throws that away. The `land` / `retire` copy is taken word for word from the phone; one verb should not be worded two ways on two screens.
  - **Execution goes only through `gtmux knowledge <verb>`**, never editing files behind the ledger's back; the change lands in `gtmux:audit:knowledge` as usual. Write operations accept only the HQ directory, so the menu bar **runs in that directory**; the cwd role gate is not loosened an inch (it blocks workers, and the person at this Mac is the commander, the same caller as the phone's gate).
  - **A window, not a popover**: the real situation board is 58KB, and cramming it into a 380pt popover is worse than not offering it; the same form and the same reason as the pane browser.
  - **The situation board reads as an outline, not the whole text** (`BoardOutline.swift` + `BoardOutlineView.swift`),
    under **the same rules** as the phone, ported straight from `mobileapp/src/screens/boardSections.ts` rather than a fresh design:
    `##` sections, `###` entries, both collapsible; the first section expanded by default (HQ pins "read this first" at the top);
    the count bubble reports **the section's own** table rows / bullets first, and only falls back to child-entry count when it has no content of its own;
    table rows (one card per pane) **collapsed by default**, a collapsed row carrying its first field, otherwise a bare column of
    pane ids says nothing. The board on this machine is 76k characters; laid out in full, finding any one item means scrolling past everything.
    The phone iterated on this three times (mobile #945 / #947 / #951) while the menu bar never caught up once;
    **one document should not read like two products on two screens**. The Swift parser tests are written item by item against the TS ones,
    so drift goes red (`BoardOutlineTests`).
  - **The situation board is rendered, not shown as source** (`Markdown.swift` parsing + `MarkdownBlocks` rendering). The first version dumped raw markdown and made the reader parse it in their head, while the phone rendered from day one: one document, two screens, two looks. The block model **copies the phone's** (`mobileapp/src/ui/markdown.ts`): heading / paragraph / list / code / quote / table / rule, inline only `code` and **bold** (measured, that is all this board uses: 1 `#`, 2 `##`, 72 `###`, one 6-column table, 50 list items, 254 lines with inline code, 45 with bold).
    - **Tables render row by row as cards**, not as a grid: that table's 6 columns are all prose, and no window width turns it into a readable grid; the same judgement and the same measurement as the phone.
    - **Blocks are built lazily** (`LazyVStack`). Laying out the whole text at once is the mistake this reading surface already made once (the first version stuffed 34KB into one SwiftUI `Text`, and switching tabs froze; `Text` does no virtualisation).
  - **A "memory" line at the top of the window**: how big, how many local snapshots, and **whether anything carries it off this disk**;
    that last sentence is the CLI's own words (`gtmux hq --records --json`), not a paraphrase, so two screens can't say different things about one fact.
    Next to it an "Export…" that uses the system save panel. This window **sits right on top of that data** (the board and the knowledge base are files in the HQ directory),
    yet until now it was the only surface that could not export: the CLI can, the phone keeps a copy, but the app running on the very machine holding the data could not.
    The verb is **export**, not "backup": gtmux runs no backup service and does not imply that it does.
  - **The knowledge base is grouped by topic and collapsible** (`knowledgeTopics`, sorted like the phone: topics with more entries first, empty topics not listed).
    On a real machine this base is **386 entries in 7 topics**, two of them 177 and 162 apiece; a flat 386 rows is not a list,
    it is something you scroll through looking for the end. Collapsed, the whole base is 7 rows. A short "recent" block stays at the top,
    because "what did it just learn" is the first question on opening this page, and a mis-remembered lesson is not inert: it is replayed into
    every dispatch. **Collapsing does not reintroduce a cap**: the earlier line "a cap means an entry the commander can never retire"
    still stands, every entry is still in its topic. The phone **taps into a topic** (on a small screen a filtered full page reads better);
    the menu bar **expands in place**: the same information, interaction chosen per screen, not drift.
  - **HTML comments are not rendered** (the same rule on both ends). The board's opening `<!-- 写法规则:一格一句话… -->` (writing rule: one sentence per cell…)
    is HQ's note to whoever edits the board next; rendered, it becomes "the reader sees an instruction meant for someone else in the most prominent spot".
    An unclosed `<!--` **keeps the text after it**: HTML would treat the rest as comment, but the board is hand-written, and one typo would make the whole
    document vanish from that point with no explanation on screen; leaving one stray marker is the smaller failure.
  - **Knowledge entry bodies use the same renderer** (`MarkdownBody`, sharing the block rendering with the board). Entries are markdown too: evidence tables, `code` identifiers, `[[links]]` to sibling entries. Printing them raw makes the reader parse them, and **the links are this base's structure**; printed as brackets they are noise and hide the structure.
    - **`[[entry]]` renders in link style with the brackets removed** (`MDInline.link`). A single bracket is ordinary punctuation and is kept as is.
  - Data and execution both go **only through the CLI** (`gtmux hq --board --json`, `gtmux knowledge list --json`, `gtmux capture --list --json`, plus the four verbs); the menu bar **does not guess where the HQ home is**, it asks the CLI (`gtmux hq --home`), the same path `--board` takes: that path is relocatable, local or a symlink, and it should be resolved in exactly one place.
- **HQ medallion · full state parity** (menubar-hq-state-parity, replacing the early v2 "no badge"): the card avatar = a **round HQ medallion** (brand mark + "HQ" wordmark + status ring), **the same recognition token** as the phone's HQ floating disc (MOBILE §17): two screens read one HQ. The medallion's **ring colour + corner badge carry all six states**, with exactly the phone's `discState` priority (the pure parser `AgentStore.hqState`): **HQ itself waiting on you** → red ring `!` ＞ **a worker waiting on you** → red ring + count ＞ **resource bottleneck** (machine red tier, from the slow-poll `gtmux resource --json` `machine.tier`; soft amber does not count) → red ring `⚠` ＞ **HQ working** → cyan ring ＞ **all normal** → green ring. Red = needs attention (a decision or a real bottleneck), the badge says which kind; ring/badge use only the authoritative status colours (§9 `Theme.Status`), no new colour. The subtitle is still the **intelligence headline** (hq-meta-layer): one deterministically synthesised chief-of-staff sentence, "who is waiting on you + the other N are fine" or "all fine · nothing needs you", turning red in the attention states. Ring/badge are the **glance layer**, the headline the **sentence layer**; they complement each other. **No more fleet pip strip** (anonymous dots duplicating the list/summary), and **no more whole-card amber outline** (state now lives on the medallion ring; a worker waiting on you is unified from the old "amber" to red, matching the phone disc and the radar's waiting). The status-bar count still includes HQ, but **notification / push / lock-screen tallies exclude HQ**; **HQ no longer pushes any notification the way a worker does** (#557); a separate notification category in the chief-of-staff voice remains to-do (see hq-meta-layer). With no HQ, the avatar = a **greyed medallion** + "未运行·点击启动" (not running · click to start; shells `gtmux hq`).
- **The card expands into the chief-of-staff report** (menubar-hq-report, 2026-09-14; mockup `mockup/menubar-hq/`): the head is unchanged and its click still jumps to the HQ pane, but a ⌄ at its right end opens, INSIDE the bordered panel, **the phone's report table** (MOBILE §17, `hqHeaderModel.ts`), ported not redesigned (`HQCardReport.swift`, pure, pinned in both languages): a key column (`machine · knowledge · board · HQ did`, en 80 / zh 48 wide, measured, one width for all rows) and a value; the rows that are doors carry a `›` and open the reader window on their tab. **A row with nothing to say is absent.** `knowledge` folds the phone's "owed" row into the door (`478 entries · 7 waiting on you · oldest 16d`; the popover has 420pt, the phone has two rows), amber **only past the two-week line `gtmux doctor` uses**. `HQ did` is the last day's tally of the supervision's own acts (`gtmux events --since 24h --acts`, read from the app's own cwd so it never advances HQ's watermark), in the phone's fixed order (consequence, not count). **The machine row leads only when it is red**, and then it is red and may wrap to two lines (a truncated reading is worse than a wrapped one, and it is the reason the card opened); at amber it keeps its ordinary place in amber; healthy it is plain grey and still a door. Why this and not the two other directions drawn: three labelled doors in a permanent row under the card would spend 30pt of the popover's most expensive pixels standing (what this section refused once already) and leave no room for owed / did; a standalone HQ window mirroring the phone page adds a click to the jump — the most frequent act — and needs CLI reads that do not exist yet; the table here is that window's left column if it is ever built. **The card opens itself on ENTERING an attention state** (HQ waiting · a worker waiting · a resource bottleneck), once — leaving red does not close it, staying red does not reopen what the reader closed; a manual toggle is remembered (`hq.cardExpanded`). All normal: one line, as before. The banner's two icons stay as shortcuts; the labelled rows are the discoverable way in (2026-09-14: 「kb 与 board 不太明显」). **Data only while open**: the expansion polls its four reads on expand and every 60s while the popover is open; collapsed it costs nothing beyond the resource poll the medallion already runs, which now keeps the whole snapshot instead of the tier alone.
- **The export is locked, and the sheet asks for the lock before the place** (hq-export-passphrase, 2026-09-14; mockup `mockup/hq-export/`): Export… opens a sheet — passphrase, once more, a Show toggle, and ONE hint line that names the one thing to fix ("too short", "the two differ", "good") — and only then the save panel, offering `gtmux-hq-<date>.tar.gz.age`. Deciding the lock after naming and placing the file, then being refused for a short passphrase, is the wrong order. A checkbox keeps the passphrase in this Mac's keychain, on by default: the lock is for the copy that travels, not a chore for the person at their own keyboard (the commander: 「电脑是自己的，不应该给自己设置太多麻烦」), so the second export opens on a single line — "using the passphrase in this Mac's keychain · Change…" — and is one click. The sheet ends on a confirmation page: the path, the size, and the sentence saying how it opens (`gtmux hq --import`, or any age tool) and that gtmux keeps no copy; on failure, the CLI's stderr verbatim. The passphrase reaches the CLI over stdin, never argv. The records line gains "last export 3d ago, locked" — the one off-machine copy gtmux can vouch for. Nothing else changes: the base on disk, the snapshots and the board stay as they are, and no surface nags about what the base holds.
- **The card's `usage` row and the reader's Usage tab** (menubar-hq-usage, 2026-09-14: 「这里没有 usage 的信息，跟 app 里感受不一样」). The phone's header has a usage door; the Mac card lacked it. Now a `usage` row sits between the board and the machine: **one window per plan, the tightest** (`claude Fable 49% · codex wk 0%`), the phone's `tightestPerPlan` + `planLabel` ported, absent when no plan is readable; its door is a fourth reader tab, Usage, the phone's UsageSheet (MOBILE §17.2) on the Mac: a lead card with the tightest non-session window and its reset, quotas grouped by agent as neutral bars (colour is status only), tokens by day (usage-daily-totals: today and this week across every agent, a seven-day bar chart, the week's split per agent — in place of per-session lifetime output), sessions sorted by trouble (alerted → burn rate → context share; the parked fold into one count row). The machine is its own tab here rather than a section. Reads `gtmux usage --json`, which is the cached probe, only while the tab shows.
- **The reader window has a third tab, Machine** (same change): `gtmux resource` laid out to read — four readings (memory · disk · load · power), the core's own warning sentence under them (the medallion's ⚠ finally has somewhere on the Mac to say what it is: 「当有资源告警的时候，menubar 并无法显示情况」), the per-agent RSS/CPU table heaviest first with the session name beside the pane id, and the orphans the core calls reclaimable with its own hint text. **Reading only** — no kill, no reclaim: that would be the first act in this window that touches the machine, and the window's rule above (no driving) stands; if wanted it is its own change with its own confirmation and audit record. Read only while the tab shows.
- **Popover height grows with the fleet** (2026-08-11, corrected 2026-08-12): list height = **measured content height**, capped at
  `min(820, usable screen height − 24 − measured chrome height)`. Before, only `.frame(maxHeight:)` was set, and a SwiftUI
  `ScrollView` **has no intrinsic content height**: it is "satisfied" at any size, so inside a content-sized
  `NSHostingController` it collapsed to the minimum and the generous cap was never used: a screen with room for 17 rows showed 3.
  **The cap answers "how tall may you be", and nothing was answering "how tall do you want to be"**. The content has to report its own height before the popover can grow to it.
  - **But letting SwiftUI alone grow pushes the panel off screen** (the regression measured in v0.50.0): `NSPopover` positions its window by
    **`contentSize`**, and a SwiftUI view that resizes itself inside an `NSHostingController` **never updates that property**, so the popover kept
    positioning by the original size and all the extra height overflowed **above** the screen. Measured (12 agents, 1728×1117): the panel measured 732pt,
    `contentSize` stayed 320×320, the window was 758pt tall with its top edge at y=1501 while the screen ends at 1084: the header, the HQ card and the
    first rows were off screen. Note what these numbers rule out: 732 fits comfortably in a 983 budget; **the panel was never too tall, it had just never been "declared"**.
    So the panel must **report its measured height to the host** (`PanelSize`), and the host sets `popover.contentSize`.
  - **Chrome must be measured, never a constant**: the HQ card exists only while HQ runs, the update bar only when a new version exists;
    any guessed allowance is wrong under some combination. The cap is computed from the measured chrome (`PanelMetrics`, a pure, testable function).
  - **An open panel that changes size does not re-anchor**: measured with 300 sessions, the window landed at y=-3504 on first open and was only
    rescued about 0.5s later by an incidental resize. So check once after a size change, and **re-`show(relativeTo:)` only when it is really off screen**:
    this is a repair, not a routine step, because the panel can change height on every poll, and every reopen resets the selection the user is
    walking with the arrow keys. The test's **upper bound is the full screen frame, the lower bound the usable area** (the bubble's arrow sits under
    the menu bar by design); using the usable area for the upper bound would judge a correctly placed panel as off screen and reopen it forever.
- **Judgement in the core, wording at the edge** (hq-verdict-single-source, 2026-08-11): the six-state verdict is decided by the Go core
  (`internal/radar.hqVerdict`), attached to the digest's supervisor row; **no rendered sentence is sent down**: the headline must be localised and
  each surface has its own language state, so a sent-down string would override the reader's choice. Because the menu bar reads the fast-poll
  `agents --json` (the verdict rides on digest, and sampling the machine on every fast poll is exactly the cost its separate slow timer avoids)
  it **still parses locally**, but `testHQStateMatchesCoreVerdictOrdering` aligns it with the core case by case; this incident was precisely
  "one rule written twice, silently diverging": with the machine in the red tier the menu bar said "机器资源紧张" (machine resources tight) and the phone said
  "都正常" (all fine). **The two screens' different forms are deliberate** (card vs disc, see MOBILE §17); the coordination comes from one token +
  one verdict, not from looking alike. Don't "fix" that difference in passing.
- **The command deck is on the phone / web, not in the menu bar** (the menu-bar card only jumps to the pane). Its structure is defined by MOBILE.md §17 / WEB.md §10; don't copy the details here. **Copying once caused drift**: early mobile did have a "fleet situation board (`/api/digest`)" zone, which `hq-command-page` deleted (the list belongs to the radar; the HQ page answers only what the radar can't: judgement / your call / activity / conversation). The menu-bar side does not change.
- **Red line**: the command deck talks only to HQ, and HQ drives the fleet (`send/spawn`); it advises, it never decides for you. To reach a worker's Detail, go through "open session" on a "your call" decision card (no longer by long-pressing a fleet row; that fleet-row list was deleted with `hq-command-page`).

## 13. Preferences: Anywhere + Sharing · §13 mockup

Two orthogonal tabs:
- **Remote access (the shared base, applying to Pair and Share alike)**: LAN is free; the Standard tunnel is free (a stable hosted address, set up once); **the Direct tunnel = unlocked by redemption code (a paid purchase)**, running over your own VPS + domain (self-tunnel), for networks that block Cloudflare; once unlocked it is an optional backend.
- **The identity layer**: **Pair** = your own devices, full power = everything the menu bar can do (**including HQ**); **Share** = collaborators, per-session grants of "visible / input" (input implies visible), per-link scope, revocable, never including HQ or Preferences. Enforced server-side. Copy standardised on "visible/input"; flat icons (geometric shapes + monospace chips, no emoji).
- **Whole-window layout (aligned with Preferences.swift)**: a 460-wide grouped form: General / Status bar / Notifications / **Remote access** (an access `关闭|局域网|任意网络` (Off|LAN|Anywhere) segmented control + address subtitle + tunnel `标准|直连` (Standard|Direct) + a live "currently connected" list, hidden when empty) / **My devices · Pairing** (device list + revoke + Pair a new device…) / **Sharing** (master input switch + share-link list with per-link expandable scope editing + New share… + a new link shown only once) / Software update (normally folded to a single version line). Switching to "Anywhere" first shows a long-lived-exposure confirmation.
- **Two sheets**: PairDeviceSheet = one one-time code (5 minutes) over three media: the phone scans a QR / the browser opens `url/#c=code` / the terminal runs `gtmux attach`; **the ⚙︎ menu's "Pair a device…" and the empty-state CTA go there directly (not through Preferences)**. The sheet always shows an access status bar at the top (mode + address + switch); when it detects remote access is off it runs a pre-step first (choose LAN/Anywhere, and under Anywhere optionally the **tunnel backend Standard/Direct**, Direct greyed out until unlocked → clicking "Enable" only opens the door, the pairing code is produced by ②), self-repairing without bouncing back: Preferences is the management surface, the sheet is the task flow. The status bar includes the backend (e.g. "任意网络 · 标准", Anywhere · Standard). NewShareSheet = name + per-session "visible/input" checkboxes, one step to build the link (input greyed out until visible is checked; at least 1 visible to create); after creation it flips to the **delivery page**, the same one-code-three-media shape as pairing: the collaborator scans a QR / opens `url/#g=code` in a browser / runs `gtmux attach` in a terminal; the three entries share one link and one scope, and the full token is shown only once.

## 14. Popover action re-layout · §14 mockup

The footer's 5 peer icons → three layers: top HQ entry · connection bar (remote/share state always present) · action group (Overview/New/Restore/Live) · bottom (Preferences + version). Pair folds into the connection entry.


## 17. Server mode · server-mode (stay awake with the lid closed)

Closing the lid puts macOS to sleep, the tunnel drops with it, and the agent's turn freezes on the spot, so "commanding from the phone" stops
working the moment the lid closes. Server mode is the switch for that. Underneath is an **undocumented Apple** kernel setting
(`disablesleep` → `IOPMrootDomain.SleepDisabled`), which needs one administrator authorisation; measured, it **keeps serving with the lid closed,
and even unplugged, walking around on battery** (command `gtmux awake`; openspec change `server-mode`,
`docs/design/server-mode-research.md`).

**The indicator is a thin ring on the existing logo, not a second status item** (corrected 2026-07-31: the earlier implementation added a separate
`NSStatusItem`, so the menu bar showed two gtmux icons, which reads as two apps installed, and is wrong):

- **On the existing brand mark**: while awake, a **5.5pt red dot** is overlaid at the centre of the mark, with a 1pt inverse halo
  (so it stays distinct when the mark itself is red), **breathing slowly on a 2.4-second cycle** (alpha 0.55↔1.0).
  One gtmux exists, only its state differs; two icons = two apps.
  (Evolution: a thin frame around the edge → a baseline bar → the red dot. The first two were too subtle; at 18pt they looked like
  a rendering artefact or failed to read as "still running".)
- **This is an explicit exception to §9 "colour expresses status only" and §10 "zero motion"**; the reasoning is under "why this red dot is allowed" below.
- **Why this red dot is allowed** (decided 2026-07-31): the recording indicator is the **one visual language every user already reads**:
  "something is still running, and you started it". And server mode's biggest risk is precisely **being forgotten**. A symbol everyone understands
  beats a shape that is more "by the rules" but that nobody can read.
  **The exception is boxed in by three constraints, so it cannot be misread as "an agent is waiting on you"**:
  ① it is **a small dot at the centre** of the mark, while waiting turns **the whole mark red**: completely different silhouettes in peripheral vision;
  ② the breathing is **slow and shallow** (2.4s, alpha 0.55↔1.0), read as "alive" rather than "alarm"; the rest of gtmux has zero motion,
     so this one movement itself says "this is an ongoing state";
  ③ the inverse halo keeps it visible on a waiting-red background.
  **The timer exists only while enabled** and is destroyed on disable; an idle gtmux still draws not a single frame.
- **Present only while on; gone the moment it is off.**
- **The menu bar no longer offers an off switch**; see "one control surface" below.
- **The popover header echoes it in one phrase**: after the summary line, `· 合盖不睡` (stay awake with the lid closed), only while on, red when the guardrail trips.
  **Read-only**: it is "a notice where the eye lands", not a control.

**The authorisation card** (§5 tone: plain, matter-of-fact, no marketing voice, en+zh) must explain three limitations, **with live numbers filled in**:
battery (sleep resumes at 20%, a reminder at 30%, how long it lasts at current draw) · heat (worse cooling with the lid closed, **naming the fanless
Air**) · exposure (remotely reachable while unattended for long stretches; the screen lock is unaffected and still locks per system settings).
Plus one line, **"stays on until you turn it off yourself"**: a hard requirement of the spec, standing in for the auto-expiry that was removed.
On an **unverified macOS** add a line "unverified ≠ unsupported" and change the button to "I understand, enable anyway".

**Known limitation (evaluated 2026-07-31, decided not to fix)**: the system's administrator prompt shows the name **osascript** and a generic
lock icon, with no gtmux branding. The reason is that the process requesting authorisation is osascript (menu-bar app → CLI → osascript).
The only way to make it show the gtmux icon is **to have the menu-bar app request authorisation in its own process**
(`NSAppleScript`), but that scrapes against the architectural iron rule "the menu-bar app is a pure consumer of the CLI", and gambles on the
hardened runtime not blocking it (Apple Events have a record, see memory `app-automation-entitlement`).
**Trading an architectural red line for an icon is not worth it.** The authorisation card has already said everything before the prompt appears,
and that is where the trust is carried. If Apple ever offers a way that doesn't sacrifice the layering, revisit.

**Turning off never asks for a password** (corrected 2026-07-31). Earlier versions popped the administrator dialog on every disable, just so it
took effect "immediately": a password box in exchange for a few seconds, placed in front of **the one action that must never fail**. Now it writes
an unprivileged stand-down marker, and the daemon responds within a second via launchd `WatchPaths`; it escalates only when the daemon is missing
(deleted, or the setting was never gtmux's to begin with). **When the daemon restores sleep it also clears gtmux's ownership record**, or a normal
disable would be read by the next status read as "expired".

**One control surface: the Mac's Preferences** (decided 2026-07-31). A `?` tooltip next to the group title explains "what server mode is"
(one sentence, no permanent real estate); the explanation card before enabling is a **custom-drawn sheet** (420pt,
`ServerModeConfirmView`), not an `NSAlert`:
the latter wraps too much when narrow, and once an `accessoryView` widens it the system switches to a **centred layout**, so the title is centred
and the body left-aligned, a mixed alignment. Only a custom sheet guarantees one alignment, one type scale and the same look as Preferences; bullets
use a hanging indent, wrapping aligned with the body rather than falling back to the left margin.
It is **its own group**, placed **before**
Remote access / Pairing / Sharing: those three are a continuous "door + identity" set that must not be split down the middle,
whereas server mode is **a property of this machine** (awake or not), not "who may come in".

**The phone only shows it, never controls it**: the state appears as a thin ring outside the connection status dot (MOBILE §18), with no switch.
The reason is not a capability limit but that **every management path of this feature ends in "type an administrator password on the Mac once"**;
a remote switch that still sends you back to the computer is worse than none.

**Red lines**: server mode **never enters the radar** (it is not an agent), **never changes the agent status item**, **never touches the screen lock / auto-login /
FileVault**, **can never be enabled remotely**.

---

## 16. Icon sizes — a hard requirement (decided 2026-08-12, machine-checked)

**Background**: this is not an aesthetic preference, it is a mistake made over and over. The magnifier in every pane-browser search field, the rows
of 9pt chevron / eye / xmark in the popover: all "an icon smaller than the text next to it". Once or twice is an oversight; twenty-two places is
systemic: hands writing SwiftUI reach for a small number by reflex. So it becomes **a rule + a machine check**, no longer left to discipline.

**Rules (apply to every SF Symbol in the menu-bar app)**

1. **Icons are always SF Symbols; never use text characters as icons** (`⌕` `✕` `⚙` `▸` and the like).
   A text glyph's **ink size has nothing to do with its point size**: `⌕` at 13pt looks smaller than the 12pt placeholder text next to it,
   which is exactly the spot the commander pointed out. An SF Symbol's point size is its optical size.
2. **Minimum 12pt**. In any `Image(systemName:).font(.system(size: N))`, `N` **must not be below 12**.
   (**Enforced by check-design.sh**, see below.)
3. **An icon must not be smaller than the text it accompanies.** A labelled icon's symbol size ≥ the label's font size;
   the **leading icon** of an input field / toolbar takes 13pt (one step above 12pt body), because it has to be seen first.
4. **A clickable icon's hit area is ≥ 22×22** (`.frame(width:height:)` + `.contentShape(Rectangle())`),
   decoupled from the visual size: the icon may be restrained, the hot zone may not.
  - **The marker is a prefix, so matching must tolerate a prefix** (fixed in v0.51.1, learned the hard way): `gtmux focus` finds the terminal tab by
    **title-prefix matching**, and `tab-alert` puts `● ` at the very front of the title, so on v0.50.0/v0.51.0 "clicking the waiting row in the menu bar
    doesn't jump", and **only waiting rows broke** (only they are marked), exactly the ones the user most wants to click. The fix is not about `●`:
    **terminals add their own decorations too** (Ghostty adds a glyph to background tabs that rang the bell), so matching should be tolerant by nature;
    `ghostty.TitleMatchesSession` strips the leading non-alphanumerics before comparing.
    Details and the two AppleScript traps are in `docs/TROUBLESHOOTING.md`.

5. **Exemption: the status badge** (`StatusBadge`). It is a shape we draw ourselves, its size dictated by §1's status-language table
   (the inline 9–11pt is **deliberate**, because it always sits against a larger identity avatar and is not a standalone icon).
   This is the only exemption, and it must go through `StatusBadge`; no other hand-drawn small icon may be used to get around this rule.

**The limits of the machine check (stated honestly)**: `check-design.sh` can only read the **declared point size**;
it cannot judge optical size, nor rules 3 and 4. It guards the rule-2 floor plus rule 1's known offending characters.
"Is this icon big enough here" is still a review judgement; the floor, though, no longer depends on memory.
