# hq-promotion-exit — a charter-level lesson gets a mechanical exit into the gtmux repo

Origin: phase 3 of the commander's audit-trail program (phase 1: #808 journaled the
supervision's acts; phase 2: #810 gave knowledge entries provenance). The trigger is
the live HQ's own words (journal seq 15704): "self-audit has been running all along —
what's missing is the EXIT: these conclusions have no mechanism to flow back into
gtmux's seeds or specs; I can only keep them in local notes, and a successor lives
off a hand-written board."

## Why — "FLAG it" has no flag

The charter has demanded the exit for a month without providing one:

- The correction-to-charter loop says a charter-level lesson must be "FLAGGED for a
  seed/spec update rather than only noted locally" — but a flag with no mechanical
  form is a note. The live HQ invented its own topic for them (`charter-flags.md`,
  52 KB) and accumulated **34 items, 16 marked high-priority**, of "historical HQs
  actually hit this, gtmux should change, never did".
- The rot is measured, not hypothetical: the `hq-unread-noise` change was born only
  because the commander MANUALLY ordered "move the backlog from HQ's notes into the
  repo" — and its audit then found the ledger's own C7 estimate off by ~50× (2 % of
  the debt, not the headline share). Un-carried flags do not just rot; they drift.
- `hq-charter` (07-13) already named the promotion test — "does it hold on another
  machine?" → PROMOTE into seed or code, else KEEP local — and left promotion as
  prose. Phase 2 built exactly the substrate a mechanical exit needs: entries with
  provenance that can cite their evidence.

## What changes

**Promotion becomes a ledger lifecycle with an artifact.** Two new operations on a
live knowledge entry, through two new `gtmux knowledge` verbs (mutations, HQ-home
gated like every other):

| Verb | Does |
|---|---|
| `promote <id> --why "…" [--target "…"]` | marks the entry charter-level and writes a PROMOTION BRIEF — a rendered, self-contained hand-off document under `knowledge/promotions/` carrying the lesson, the why, the suggested landing target, and the entry's full provenance |
| `land <id> --ref "…"` | closes the loop when the lesson actually lands in the gtmux repo (a PR / spec / seed change), naming the reference; the brief is removed, the ledger keeps the whole lifecycle |

`gtmux knowledge promotions` (read-only, open to anyone) lists the pending queue,
headed by the count and the OLDEST brief's age — the exact anti-rot instrument
`charter-flags.md` never had. Topic renders badge a promoted-pending entry (⚑) and a
landed one (→ ref), so the state is visible where the lesson lives. Both operations
journal through `gtmux:audit:knowledge` like every other mutation.

**Rot is surfaced where health already lives.** `gtmux doctor`'s HQ maintenance
section gains a `knowledge promotions` row: quiet when the queue is empty or young,
flagged when the oldest pending brief has stood past its floor — because a promotion
nobody carries is precisely the failure this change exists to end, and it is
otherwise silent. The distill ritual's teaching adds the queue to its checklist, so
the existing periodic clock (not a new one) keeps it warm.

**The playbook (v25) finally names the verb.** "FLAG it for a seed/spec update"
becomes `gtmux knowledge promote …`; the correction loop and the distill ritual
teach the full lifecycle including `land`, and teach migrating the existing
`charter-flags.md` backlog through it (promote what still holds, dismiss the rest —
judged, not bulk-imported).

## What the brief deliberately is NOT

- **Not an openspec change generated into a repo.** gtmux the product runs on
  machines with no gtmux checkout, and the HARD role whitelist keeps HQ out of
  repos. The brief is the evidence package a human — or a worker THEY dispatch —
  carries across; writing repo files stays a human/worker act.
- **Not auto-dispatched.** "No new HQ-auto-dispatches-on-its-own autonomy"
  (hq-dispatch) stands: gtmux surfaces the queue; the commander decides who lands it.
- **Not a new wake class.** The doctor row plus the distill ritual's existing clock
  carry the surfacing; a class-table entry is not warranted until measured need.

## Non-goals

Bulk migration of `charter-flags.md` by gtmux (HQ judges it entry-by-entry through
the verbs); promotion of LEGACY (pre-ledger) lessons directly (migrate to an entry
first — that is one `add`); repo-side tooling for consuming a brief.

## Impact / risk

The risky direction is a queue that silently never drains — the charter-flags rot
in a new costume. That is why `land` is a first-class verb whose absence is VISIBLE
(doctor row, list header age, render badge), and why every slice pairs a "pending
surfaces" test with a "landed clears" test. The playbook's corrections/distill
sections change, so `hqPlaybookVersion` bumps 24 → 25.
