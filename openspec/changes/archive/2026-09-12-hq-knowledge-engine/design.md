# Design — hq-knowledge-engine

Every numbered decision names the alternative it rejected and the evidence, so the next
iteration can see what was decided and why rather than re-deriving it. Survey behind the
choices: `docs/design/knowledge-engineering-research.md`.

## D1 — One leaf package, extracted by a pure move first

`internal/knowledge` owns the ledger and everything derived from it. `internal/hq` keeps
supervision (sensors, playbook, verb dispatch shims). Import rule stays acyclic:
`app → hq → {knowledge, mine, radar, dispatchbridge} → leaves`; `knowledge` imports
`state`, `events`, `i18n`, `usercfg` and nothing above; `mine` does not import
`knowledge` (leads are plain structs; hq maps them into the pool).

Phase 1 moves the eleven files with no behaviour change and a test that the rendered
topic files, the JSON index and every CLI output are byte-identical before and after.
Rejected: refactoring while moving. The `decompose-app-package` change showed that a pure
move reviewed in one PR is the only way a 3.5k-line extraction stays reviewable.

Layout (files, not sub-packages — one package keeps the ledger's invariants in one place):

```
internal/knowledge/
  ledger.go        record format (v2 additive), read/append, migration of v1 records
  kinds.go         kind vocabulary + topic tags; validation
  provenance.go    provenance kinds + counts; the recurrence feed-back
  audience.go      audience vocabulary; carrier resolution per audience
  pool.go          candidate pool (capture + mined); neighbour grouping for --list
  distill.go       add/supersede/retire/dismiss with the ACE constraints
  promote.go       promote / land / withdraw; brief rendering
  lint.go          orphan / broken-link / duplicate / stale / assumed-kind checks
  neighbours.go    kind + keyword-overlap similarity (no vectors)
  render.go        deterministic topic and canonical-file rendering
  distribute.go    managed block install/refresh/verify per agent; repo block
  api.go           index/entry/pool shapes shared by CLI, serve, mac, phone
```

## D2 — Kind: CoALA's split, one level finer

`facts` (semantic: how this machine / account / world is), `howto`, `pitfalls`,
`judgment`, `decisions` (procedural, three shapes plus the "why we chose" record).
Rejected: keeping the six topics — `corrections` is a source and `accounts` a subtype;
rejected: PARA / Johnny-Decimal — they classify by actionability and filing, not by what
a thing is. `topic` survives as free tags so `gtmux knowledge topic` and HQ-declared
vocabularies keep working; a kind is required, a tag is optional.

## D3 — Provenance is a field with a count, not a topic

`correction` · `recurrence` · `mined` · `capture` · `self`, plus `count` and the last
seen time. A filed lesson that is hit again bumps `count` (the miner's recurring-error
tally and the correction lexicon feed it) — that is ACE's helpful/harmful counter and the
signal that the CARRIER failed, not the memory. Rejected: a `corrections` topic — it
classifies by who said it, which Zettelkasten calls a mistake and which hid that one
correction produced a pitfall and a workflow.

## D4 — Audience replaces `--target`

Four values, answering "who must know this": `hq`, `machine`, `repo <path>`,
`everyone`. HQ must choose one to promote; the UI shows the word, not a path. Rejected:
free text (the measured result was 45 flags with "gtmux 仓库" in prose and no exit);
rejected: more than four (a team runbook is a `repo` or `machine` carrier by another
name). The design doc `knowledge-layers.md` is rewritten around these four.

## D5 — Machine distribution: one canonical file, index-only managed blocks

gtmux renders `~/.config/gtmux/knowledge/machine.md` from the ledger (audience
`machine`, live entries, each with its exemplar). Into each supported agent's global
instruction file it maintains a block:

```
<!-- gtmux:knowledge:begin v1 <hash> -->
gtmux 沉淀的经验（本机所有 agent）· 全文见 ~/.config/gtmux/knowledge/machine.md
- <kind> · <title> — <one line>
…
<!-- gtmux:knowledge:end -->
```

Index only, never the full text: the block cannot grow into every session's context
(Skills' progressive disclosure), and every agent can read the canonical file when it
needs the why. Rejected: `@import` pointers — Claude Code supports them, Codex,
opencode and Kimi do not; rejected: full text per agent — four copies drift, which is the
exact shape the flags list rotted in. The block hash lets doctor tell "stale" from
"hand-edited" from "missing". Carrier paths per agent come from the agent registry
(`internal/agents`), verified against each agent's documentation as a task, not assumed.

## D6 — Repo and everyone

`repo`: the same managed block in the repository's `AGENTS.md` (or `CLAUDE.md` when only
that exists), with full text because it is small and local; gtmux never commits — the UI
says "written, not committed". `everyone`: a GitHub issue prefilled from the brief (URL
scheme, no token) or copy; land ref is the issue URL; these briefs are exempt from the
overdue floor. Rejected: telling users to open a PR — of 6 pending briefs 5 named the
repo and none could be acted on by a non-maintainer.

## D7 — Withdraw, and the hypothesis status

`withdraw <id> --why` moves promoted → live. Rejected: `retire` (kills a correct entry)
and `land --ref local-only` (a lie in the audit trail). `hypothesis` is a status for a
filed but unverified lead: it renders in its own section, is excluded from distribution,
and lint asks about it after a floor (~30 days). Rejected: keeping unverified leads in
the pool — the pool is HQ's inbox, not a shelf.

## D8 — Lint and neighbours without vectors

Lint checks: orphan (no links in or out), broken `[[link]]`, near-duplicate titles,
stale (superseded-by pending, hypothesis past floor, promoted past floor unless
`everyone`), kind-assumed-at-migration. Runs by hand and inside self-check; output is a
report, never an edit. Neighbours: same kind first, then keyword overlap on
title + body; used at intake (`--list` groups the pool), on `add` (shows the closest
three so `supersede` beats a near-duplicate), and by `neighbours <id>`. Rejected:
embeddings — cgo-free CLI, and the measured need (11 of 146 leads were one family) is met
by overlap; revisit only with evidence.

## D9 — The two ACE constraints on distill

distill never rewrites a topic file wholesale (context collapse) and never shortens an
entry on merge without `supersede` keeping the old text (brevity bias). Both are already
true of render and supersede; this change states them as requirements with tests.

## D10 — Migration is read-time and additive

Old records are mapped at read; the mapping is a table in `ledger.go`; lint flags every
assumed kind until HQ confirms with `supersede` or `knowledge kind <id> <kind>`. No file
is rewritten in place; a backup of the ledger precedes the first write in the new format.
