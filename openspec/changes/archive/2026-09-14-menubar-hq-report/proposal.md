# Change: menubar-hq-report

> STATUS: implemented 2026-09-14 (same PR). Proposed 2026-09-14, authorised by the commander 「按你的建议来吧。注意中英文都要支持。」 after the design canvas (direction A of three). Design record: `design.md` here.

## Why

The menu bar's HQ card says one sentence and offers one click. Measured against what the
commander asked on 2026-09-14 (「目前 kb 与 board 不太明显，而且当有资源告警的时候，menubar
并无法显示情况，点击只能跳到 hq session」):

1. **The two documents are invisible.** The situation board and the knowledge base open from
   two 12pt icons in the dimmest grey at the right end of the role banner. They carry no
   number, no name, and nothing that says "there are 7 entries waiting for you in here".
2. **A resource alert has nowhere to say what it is.** At the red tier the medallion shows `⚠`
   and the headline reads "machine under pressure" — and that is all the Mac app can show,
   on the machine whose memory is the problem. The click jumps to the HQ pane, which is a
   terminal, not a reading of the machine. The phone at least has its usage sheet; the Mac
   app has nothing.
3. **The phone already solved this shape.** Its HQ page's standing header expands into a
   chief-of-staff report: what is owed to you, what HQ did, the machine when it matters,
   and the two document doors as rows of one table (MOBILE §17, `hqHeaderModel.ts`).
   The menu bar never caught up, so the two screens read one HQ differently.

## What Changes

1. **The HQ card expands in place into the phone's report table.** A disclosure at the
   right end of the card head (⌄ / ⌃) opens a key/value table INSIDE the bordered panel:
   `machine` (readings, a door to the new machine tab) · `knowledge` (count, what it owes
   you, the oldest debt; a door) · `board` (how fresh; a door) · `HQ did` (24-hour tally of
   the supervisor's own acts). The card head's click still jumps to the HQ pane — the most
   frequent action and the keyboard's ⏎ are untouched. Rows follow the phone's rules: a
   row with nothing to say is absent; the machine row leads and turns red only at the red
   tier (amber at amber); only the knowledge row may turn amber, and only past the
   two-week line `gtmux doctor` uses.
2. **The expansion opens itself when the card turns red**, once per entry into an attention
   state (HQ waiting · a worker waiting · a resource bottleneck); a manual toggle is
   remembered across popover openings. All normal = one line, as today.
3. **The reader window gets a third tab, "Machine"**: four readings (memory · disk · load ·
   battery), the core's own warning sentence, per-agent RSS/CPU heaviest first with the
   session name beside the pane id, and the orphan processes `gtmux resource` names as
   reclaimable with its own hint text. Reading only: no kill, no reclaim — the window's
   rule (DESIGN §12) stands.
4. **`gtmux events --acts`** narrows the stream to the supervisor's own acts
   (`events.IsSupervisorAct`, the filter `GET /api/hq/events?acts=1` already applies), so a
   CLI consumer can ask "what did HQ do today" without reading the wake plumbing that
   outnumbers the acts forty to one.
5. Both languages throughout: keys, values, verbs and the machine tab follow the app's
   language setting, the same `L10n` switch the rest of the popover uses.

## Surfaces

- 终端 (terminal, incl. remote attach): `gtmux events --acts` is the one CLI change (a filter
  the API already had). `gtmux resource` is unchanged.
- 菜单栏 (menubar): the subject of this change — the card's expansion, the auto-open rule,
  the machine tab in the reader window.
- 手机 (phone): unchanged. The table the menu bar gains is the phone's own (`hqHeaderModel`
  rows + doors); the phone remains the command deck, the menu bar does not gain a composer.
- iPad: unchanged, same reason as the phone (the regular shell already shows that header).
- Web: not applicable — the shared page is a read-only mirror with no HQ page.

## What does NOT change

- The card head: medallion, headline, click-to-jump, the six-state model and its priority.
- The role banner's two icons stay as shortcuts; the labelled rows are the discoverable path.
- The red line: nothing in the popover or the reader window dispatches, sends, or kills.
- `gtmux resource --json`'s shape; the reader's board and knowledge tabs.
