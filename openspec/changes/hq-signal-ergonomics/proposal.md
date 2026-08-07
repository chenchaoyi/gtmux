# hq-signal-ergonomics — the HQ screen reads by grade: decision, attention, ledger

Origin: the commander's 2026-08-07 feedback, verbatim: 「屏幕上的信息有一些杂乱,最好能有
一定的结构与颜色,让人能更好地阅读与感知」— plus two structural observations from the same
day's HQ shift (40 h on duty, 400+ turns). Batched with `hq-unread-noise` and
`standing-wake-backoff` on the order to move HQ's backlog into the repo. Proposal only.

## Why

The HQ pane is a shared reading surface: the commander watches it, HQ writes into it,
gtmux injects into it. Three kinds of content currently look identical (HQ's own
estimate of its output mix: **~70% bookkeeping** — zero user value; **~25% intel**;
**~5% needs-a-decision** — the part that must be seen at a glance). The reader has to
parse every line to find the 5%.

Two adjacent gaps compound it:

1. **The standing "on your plate" list has no home.** Every HQ brief re-prints
   「你手上未变:A · B · C…」— the same unchanged items, turn after turn. A list that is
   re-printed verbatim in every brief is, by that very fact, a PERSISTENT VIEW being
   simulated in a scrolling transcript. `gtmux tasks` already holds the ledger
   (tier/priority/surfaced/disposition); what's missing is a standing
   "awaiting-commander-decision" view that updates only when its state changes.
2. **Tick briefs restate snapshots.** The playbook bounds the brief (≤6 lines) but does
   not require it to be a DELTA, so an unchanged fleet still earns a re-stated summary
   and the unchanged plate list rides along every time.

### What "color" can and cannot mean here (feasibility boundary, stated up front)

- Wake lines are typed INTO the HQ agent's composer as input text; HQ's replies render
  through the agent's own TUI. **Neither can carry ANSI color** — gtmux does not own
  that rendering. The in-pane vehicle for grade is therefore STRUCTURE: a small,
  encoding-robust GLYPH vocabulary at a fixed position, which the eye can scan the way
  it would scan color.
- Surfaces gtmux DOES render — `gtmux events --follow`, `gtmux tasks`, `gtmux digest`,
  doctor — CAN color, and SHALL (tty-gated, `NO_COLOR`-respecting).

## What changes

1. **A three-grade attention scale, derived not invented.** Every wake class and ledger
   entry maps deterministically onto three grades:
   - 🔴 **decision** — needs the commander: irreversible, or carries money/safety
     consequence, or an explicit ask (`waiting·kind`, `asks`, CRITICAL degradations);
   - 🟡 **attention** — a line is blocked or changed in a way worth knowing
     (waiting-family, stalls, notable fleet changes);
   - ⚪ **ledger** — bookkeeping: recorded, pullable, zero interrupt value.
   The mapping lives in ONE place (the class table / severity classifier already exist —
   this adds a projection, not a new judgment) and is exposed wherever the class or
   severity already is.
2. **Wake lines carry the grade** (`hq-wake-protocol`, MODIFIED signal-language
   requirement): the `» gtmux·<class>` grammar gains a fixed-position grade glyph,
   pinned by the existing fixture tests, encoding-robust like the rest of the line.
3. **HQ's register speaks the same scale** (`supervisor-agent`, MODIFIED signal-register
   requirement): the ⟣ vocabulary maps onto the three grades so the pane scans
   uniformly; the playbook's print gate becomes grade-explicit — ledger-grade content
   goes to the board/ledger, NOT the screen (this is the ~70% that today prints as
   prose). Playbook change ⇒ `hqPlaybookVersion` bump (repo rule).
4. **A standing pending-decision view** (`hq-attention-system`, ADDED): the attention
   ledger gains a first-class "awaiting-commander" disposition and a dedicated view
   (`gtmux tasks --pending` or equivalent — exact surface decided at implementation
   within the single-word command rule; a flag on `tasks` needs no new command). It is
   the ONE place the plate lives; briefs reference it instead of re-printing it.
5. **Tick briefs report deltas, not snapshots** (`supervisor-agent`, MODIFIED tick-brief
   requirement): a brief names what CHANGED since the last brief; the unchanged plate is
   a one-clause pointer to the pending view, never a re-printed list. (This is primarily
   HQ-side behavior — the ledger's own note — but it is written into the playbook spec
   so every seeded HQ inherits it.)
6. **gtmux-owned surfaces colorize by grade** (`hq-attention-system`, ADDED): `events
   --follow`/`tasks` render the grade in color on a tty (en/zh both), `NO_COLOR` and
   non-tty output stay plain.

## Not in scope / uncertainties

- No change to the wake channel's delivery mechanics, priorities, or classes — only
  their presentation and one new ledger view.
- The 70/25/5 mix and the ~70%-echo figure are HQ's own estimates from one shift, not
  instrumented numbers; they motivate, they don't calibrate. Thresholds/mappings are
  design-time decisions against the class table, not those percentages.
- Whether the menu-bar/mobile surfaces should mirror the grade scale is deliberately
  deferred (DESIGN.md/MOBILE.md conformance review first — 状态色 already has an
  authoritative palette; a second color axis needs a design round, not a drive-by).

## Impact

- Specs: `hq-wake-protocol`, `supervisor-agent`, `hq-attention-system`.
- Code (when implemented): `internal/hqwake` (line grammar + fixtures), `internal/hq`
  (playbook text + version bump), `internal/app`/tasks CLI (pending view, color),
  `internal/i18n` for any new user-facing strings (en+zh).
- Contracts: additive only — the wake-line grammar gains a glyph (fixture-pinned), the
  tasks ledger gains a disposition value; no JSON schema breaks.
