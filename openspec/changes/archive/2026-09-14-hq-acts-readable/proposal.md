# Change: hq-acts-readable

> STATUS: implemented 2026-09-14 (same PR). On the commander's direction, pointing at the phone's "HQ's work" section: 「hq's work 这里我不太 get 到要如何理解、利用这些信息，而且这些信息只展示两天的历史」.

## Why

Two defects, one visible and one hidden. Visible: the rows were the audit journal's own
words — `hit pitfalls/capture-pane-e-at-the-moment ×2`, `supersede a → b` — verbs a reader
has to know, pointing at nothing they can open, under no sentence saying what the section
is for. Hidden: the acts feed read a 24-hour window while the tally above it said "this
week", so "dispatched 7 this week" counted one day.

## What Changes

1. **The acts feed looks back a week** (`actsWindow` in the core, acts-only reads);
   the fleet feed keeps its day. The weekly tally is now a weekly tally.
2. **Every act reads as a sentence** the reader does not need the ledger to understand:
   「记下一条：…」「改写：a → b」「又踩到：…（第 2 次）」「晋升给 machine：…」; a dispatch's
   outcome in words (「已送达」「被草稿挡住」). The journal's own words stay for a shape the
   model does not know.
3. **Every act leads somewhere**: a dispatch or reap opens that session; a knowledge act
   opens the knowledge base at that entry. The row is checkable, not only readable.
4. **One sentence above the tally says what the section is for**: what HQ did on your
   behalf, listed so you can check it — nothing here needs handling.

## Surfaces

- 终端 (terminal, incl. remote attach): `gtmux events --acts` unchanged; the week window is the API's.
- 菜单栏 (menubar): the card's "HQ did" row already counts 24h from `events --acts`; unchanged.
- 手机 (phone): the subject of this change.
- iPad: the same section in the inspector.
- Web: not applicable.

## What does NOT change

- The journal, its record shapes, the tally's fixed order, the day/burst rhythm.
