# Handoff to the repo's Claude Code — design alignment and iteration (2026-07-18)

## What you (the user) do

1. **Overlay** this `docs/design/` onto the repo's `docs/design/` (same-named files overwrite).
   > Do not empty the directory: the repo's own `multi-agent-multi-terminal.md`, `remote-access-tunnel.md`, `DECISIONS-*` and friends are not in this bundle; keep them.
   > Exception: if the repo's `MOBILE.md` carries implementation notes newer than this bundle (the NativeTerm/chat subsections), take the union rather than overwriting.
2. Merge `CLAUDE.snippet.md` into the repo root `CLAUDE.md` (replacing the old design paragraphs there).
3. Start Claude Code at the repo root and paste the whole Prompt below.

---

## The Prompt to paste into CC

You are doing a round of **design alignment and iteration** for gtmux across three surfaces: the menu bar (`macapp/`), the phone (`mobileapp/`) and the web (`internal/server/web/`). The design authority is `docs/design/`:

Read first (in order):
1. `docs/design/DESIGN.md` (the menu-bar authority; §12/§13/§14 are this round's focus)
2. `docs/design/MOBILE.md` + `docs/design/WEB.md`
3. **§E / §F** of `docs/design/ITERATIONS-2026-06.md` (the two latest change lists, authoritative)
4. Open `docs/design/mockup/gtmux-menubar.dc.html`, `gtmux-mobile.dc.html`, `gtmux-web.dc.html` in a browser to compare pixel details (if you can't view them offline, follow the written spec).

### P0 · Priority iterations (in order, one commit each)

**1. Rebuild the Preferences window** (menubar mockup §13)
- Grouped form: General / Status bar / Notifications / **Remote access** / **My devices · Pairing** / **Sharing** / Software update.
- Remote access: a `关闭 | 局域网 | 任意网络` (Off | LAN | Anywhere) segmented control + address subtitle + tunnel backend `标准 | 直连` (Standard | Direct; Direct = **unlocked by redemption code**, runs the self-tunnel over **your own VPS + domain**) + a live "currently connected" list (the whole block hidden when empty). Switching to "Anywhere" first shows a long-lived-exposure confirmation.
- Pairing sheet: an **access status bar** at the top (mode + backend + address + switch); when remote access is off, a **pre-step** comes first (choose LAN/Anywhere, and under Anywhere choose Standard/Direct, Direct greyed out until unlocked; the only button is "**Enable**", the pairing code is generated back on the main page); the main page = a one-time code (5 minutes) via **three media**: scan a QR / browser `url/#c=code` / `gtmux attach`. The ⚙︎ menu's "Pair a device…" and the empty-state CTA go there directly, not through Preferences.
- Sharing: per-session checkboxes "**visible / input**" (input ⊆ visible; input greyed out until visible is checked); after creation flip to a **delivery page** (the same one-code-three-media shape as pairing; the `#g=` guest token is shown only once); existing link rows expand to edit scope and can be revoked; a master "allow collaborators to type" switch.
- Copy standardised on "visible/input"; flat icons (geometric shapes + monospace chips), **no emoji**.

**2. HQ · Chief-of-staff card v2** (menubar §12, the chosen design)
- Card = a "👁 CHIEF OF STAFF · 参谋长 · 统观全局" role banner + 1px outlined panel + **brand-grid avatar (no status badge)** + ~~fleet pip strip~~ **intelligence headline subtitle** (**hq-meta-layer reversed this**: the pip strip's row of anonymous coloured dots duplicated the list/counts and couldn't even answer "who is waiting"; removed, replaced by a single chief-of-staff sentence synthesised from the fleet); when HQ itself is waiting on you → **the whole card turns amber**.
- Not running = a dashed ghost strip "中控未运行 · 点击启动" (HQ not running · click to start; shells `gtmux hq`); hidden during search and in the true empty state.
- Clicking the card = **jump to the HQ pane** (no panel opens). The summary count **excludes** HQ; the status-item count **includes** HQ.
- The phone radar's HQ entry shares the same shape (mobile §17). **The phone HQ page was rebuilt under `hq-command-page`**: there is no "fleet situation board" any more; it is four zones, judgement / your call / activity / conversation (the board = what the radar can't answer; it no longer repeats the fleet). The web wide-screen command deck with three columns (situation `/api/digest` / conversation / dispatch ledger `/api/tasks`, closed to guests) (web §07) is **not yet implemented**; its situation column is to be re-reviewed together with mobile.

**3. Footer v3** (menubar §14)
- One permanent row: left "＋ New session" (icon + text on the same line) · right inline status (green dot + device count, an "input" chip, **only when true**) + version number (dim mono, always shown) + ⚙︎ menu (Preferences… ⌘, / Pair a device… / Check for updates / Quit ⌘Q).
- Contextual row "↩ Restore your last workspace · N sessions M windows": appears **only after a reboot** (a snapshot exists and no session is running); one click runs `gtmux restore`.
- The old "Reattach/New/Pair" three-cell strip and the old connection bar are deleted.

**4. Status-item icon sync** (menubar §02)
The done state carries no count; count = waiting count, else working count (`BadgeText`), **including HQ**; three display modes (dot + number / dot only / hidden when idle), chosen in Preferences · Status bar.

### Mobile round F (ITERATIONS §F; compare against the existing implementation and fill gaps)
- F1 All billing moved off the phone: no paywall; adding a server = scan-to-add as the main path (`#c=` my Mac / `#g=` guest); Servers grouped into two tracks (MY MACS / GUEST CONNECTIONS); removal = clear Keychain + revoke the push token.
- F2 Composer: resting key bar `⌨ | Tab ↑ ↓ ⏎ ⌫ Ctrl-C Esc | 常用语▾ 历史` (user-visible copy since 2026-08 is "常用语 / Quick replies"); the hard-coded 1/2/3 removed, waiting replies are handled by the **ApprovalCard** (`/api/options`, the real options 1..N); Return = newline, ↑ sends, ⤢ full-screen compose; attachments are staged before sending (upload with %, retry on failure, images go through the annotator first).
- F3 Notifications: the category's three fixed keys 1·Yes/2·Always/3·No, background `/api/send` **digits without Enter**; tap deep-links (payload carries the server name, switch server first); badge = waiting count.
- F4 Settings page: Moshi groups + PickerSheet (rows show the current value + ›); Connection/Terminal/Notifications/General/About; **owner-only items hidden for guests**.
- F5 iPad: split view at width ≥ 768, the sidebar reuses SectionList, the main area swaps in place, push deep-link = the selected row (implemented since 2026-09-12 per the rewritten MOBILE §5: `RadarPanel` + `SplitShell`, see change `ipad-universal-app`).
- F6 HQ radar entry = the chief-of-staff card (same as P0.2).
- F7 Demo mode polish (mobile §18 / ITERATIONS §F7): the entry becomes a DEMO-badged secondary card; after approval the **status arc** waiting→working→idle(latest) is visible in the radar; the HQ chief-of-staff card joins the demo (canned digest + preset conversation). The boundary rules do not move: DEMO chip throughout, no entries under Servers, exit resets, zero network.

### Web
- Tile headers state `⌨ 可输入` (input allowed; cyan, composer + digit chips) / `👁 只读` (read-only; grey, no composer + a "not authorised" line); guest permission = **per-link scope** (input ⊆ visible) + a master switch, enforced server-side; owner/guest top bars show different identities.
- Status badges gain their **glyphs** (red square with double bars / cyan loading ring / green ✓ / grey dot): the colour + shape + glyph triple encoding.

### Red lines (never cross)
- Colour expresses status only (`#EF4444/#06B6D4/#22C55E/#8E8E93`); never rely on colour alone.
- The `1/2/3` structured reply appears only in waiting, and option text comes from the agent's real prompt.
- Permissions enforced server-side; HQ advises, it never decides for you; bilingual en/zh, CJK never wraps.
- Where you differ from the mockup/spec: **report the difference before changing anything**, never deviate on your own; after each item, self-check against the mockup and output an acceptance checklist.

