# hq-knowledge-ledger — knowledge becomes entries with provenance; markdown becomes a projection

Origin: phase 2 of the commander's 2026-08-16 audit-trail program (phase 1:
`hq-action-journal`, merged #808). The live HQ's own ledger entry (seq 15704) named
the gap — "self-audit has been running all along; what's missing is the exit" — and
this change builds the substrate that exit needs: knowledge whose entries can cite
their evidence.

## Why — the knowledge base is a memory with no memory

The KB is HQ's charter-declared most important job, and it is the one artifact in
the system with no history, no provenance, and no mechanical support for its own
discipline:

- **Provenance evaporates at the merge.** `gtmux capture` auto-collects
  `pane`/`seq`/`task` on every candidate precisely so "the distill pass has
  provenance without the author re-typing it" (capturecmd.go) — and then the
  distill ritual has HQ hand-merge the lesson into markdown, where all three
  fields die. "Which events produced this lesson" is unanswerable for every entry
  in the base.
- **The charter's discipline has no mechanism.** "Update-over-append", "prune
  stale", "merge duplicates" are prose obligations on an LLM editing free-form
  markdown. Measured on the live machine: `pitfalls.md` 216 KB, `best-practices.md`
  85 KB, `corrections.md` 64 KB — monotonic growth, no supersede history, no way
  to see what changed when, or why, or what a pruned entry used to say.
- **The drain is a truncate.** The distill teaching ends "…then truncate the
  spool": accepted and rejected candidates vanish identically. A dismissed
  candidate — HQ exercising exactly the quality-gate judgment the loop exists
  for — leaves no trace that it was ever judged.
- **The successor inherits prose it cannot interrogate.** The KB is the rotation
  handoff's substance; an entry with no source can only be believed or
  re-verified from scratch. The measured 2026-08-01 dispatch-file-channel incident
  is the failure shape: a footgun "already in the KB twice", generalized into a
  rule that morning, hit again hours later — a KB that cannot say where a rule
  came from also cannot say how load-bearing it is.

## What changes

**The ledger is the authority; markdown is a projection.** A new append-only
`knowledge/.ledger.jsonl` holds the entries — one JSON op per line
(`add` / `supersede` / `retire`), each carrying a stable id
(`<topic>/<slug>`), a one-line title, an optional markdown body, and PROVENANCE:
the event seq stamped at write, an optional seq range (the distill delta), and —
when the entry consumes a capture candidate — that candidate's
`seq`/`pane`/`task`/key, inherited instead of evaporated. The topic files HQ and
`MatchKnowledge` read become deterministic RENDERS of the live entries, marked
gtmux-owned, each entry with a provenance footer.

**HQ writes through gtmux verbs, not a text editor.** A new `gtmux knowledge`
command family (public command, mutations gated to the HQ home by the same
cwd-keyed role rule as `gtmux events --ack`):

| Verb | Does |
|---|---|
| `add --topic t --title "…" [--body-file f] [--capture <key>] [--seq-range a..b]` | append an entry; `--capture` consumes the pending candidate and inherits its provenance |
| `supersede <id> --title "…" [--body-file f]` | replace an entry; the old op stays in the ledger, the projection drops it |
| `retire <id> --why "…"` | prune with a reason that survives |
| `dismiss --capture <key> --why "…"` | reject a candidate WITH a trace (journal record, no ledger op) — the quality gate finally leaves evidence |
| `list [--topic t] [--json]` / `show <id>` | read side, open to anyone |
| `render [--check]` | regenerate projections; `--check` fails on hand-edit drift |

Every mutation also appends one `gtmux:audit:knowledge` journal record (phase-1
vocabulary, trail-not-debt), so "when did the KB change, and why" joins the same
stream as everything else.

**Migration is incremental, not big-bang.** The 500 KB of existing hand-written
markdown cannot be parsed into entries and is not thrown away: the first mutation
touching a topic moves its existing file to `knowledge/legacy/<topic>.md`
verbatim (a file still equal to its seeded placeholder is simply replaced), and
the projection links to it. `MatchKnowledge` consults BOTH the projections and
the legacy files, so dispatch-time echo never loses coverage. The distill
teaching gains: migrate the legacy lessons you touch (a migration is an ordinary
`add` whose provenance says `legacy`), so the legacy files shrink by use instead
of by decree.

**The playbook (v24) re-teaches the three rituals over the verbs.** Capture
verdicts, the consult precondition, and the board/KB weld are unchanged in
meaning; "write/UPDATE the topic file" becomes the `add`/`supersede` verbs, the
drain becomes per-candidate `add --capture` / `dismiss --capture` (no more blind
truncate), and "update-over-append" becomes the supersede discipline the ledger
actually enforces the history of. The role whitelist's clause (a) gains
`knowledge`.

## What deliberately does NOT change

- **The quality gate stays HQ.** Workers still get exactly one input — `gtmux
  capture` — and no mutation verb works outside the HQ home.
- **Consult stays file-reading.** Projections are still markdown on disk; the
  charter's consult precondition and `MatchKnowledge` keep working on files.
- **The board is untouched** — it remains ephemeral, LLM-owned prose; the weld
  ("a board note never counts as capture") stands as-is.
- **No enforcement theater.** HQ can still physically hand-edit a projection (it
  owns the directory); the design makes that VISIBLE (deterministic render +
  `--check` drift detection) rather than pretending to make it impossible.

## Non-goals

- The lesson EXPORT exit (`openspec` change candidates from cited entries) —
  phase 3, which this provenance is the substrate for.
- Doctor rows for ledger/legacy stats; KB-summary injection into worker panes
  (already deferred by `hq-capture-loop`); the density-K and correction-class
  distill triggers (already deferred by `hq-maintenance-triggers`).
- Ledger compaction/rotation — curated content at hundreds-of-entries scale;
  named in Known Limitations rather than built speculatively.

## Impact / risk

The risky direction is losing knowledge or losing consult coverage. Every
migration slice pairs a "legacy content survives verbatim" test with a
"MatchKnowledge still finds it" test; bounds refuse LOUD (a title or body over
budget is an error naming the limit, never a silent truncation — knowledge is
curated content and truncation corrupts it); id collisions refuse with the
supersede hint instead of guessing. The playbook's knowledge section changes, so
`hqPlaybookVersion` bumps 23 → 24. `gtmux knowledge` is a public command and is
documented per the command-drift rule (CLAUDE.md list, `docs/cli.md`,
`gtmux --help` en+zh) — not the HIDDEN allowlist.
