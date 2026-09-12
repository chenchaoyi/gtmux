# Change: hq-knowledge-engine

> STATUS: implemented 2026-09-12 (PRs #1040–#1046 and the docs PR that archived this). Approved by the commander on 2026-09-12 ("开始").
> Design record: `docs/design/knowledge-engineering-research.md` (what was surveyed and
> why each mechanism is borrowed or not) + `docs/design/knowledge-layers.md` (the three
> layers and the exit, to be rewritten by this change).

## Why

HQ's knowledge base was built as a place to record rules. What it has become, after the
transcript miner and the flags triage of 2026-09-12, is the accumulation point of a
learning loop: observe (events + session logs of four agents) → collect (candidate pool)
→ distill (the one quality gate) → record (the ledger) → distribute → feed back. Two of
those steps do not exist, and the ones that do are scattered across `internal/hq` in a
way nobody can iterate on deliberately.

Three concrete defects, all measured on the machine this was designed on:

1. **Knowledge stops at HQ.** A lesson filed in the ledger is read by HQ and by no other
   agent. The commander was corrected on the same writing rule nine times in two weeks
   while it sat in the KB thirteen times; a hand-written flags list of 45 product defects
   accumulated because "promote" had no exit an ordinary user could take. Of the 6
   promotions pending today, 5 name "the gtmux repo" as their landing — meaningless to a
   user who never opens a pull request.
2. **The taxonomy classifies by the wrong axis.** `corrections` is a SOURCE, not a kind of
   knowledge (a correction yields a pitfall or a workflow); `accounts` is a subtype of
   `environment` holding a top-level slot; decisions and judgment heuristics have no
   home and live in PRs. Nothing marks a lead as "not yet verified", so a mined lead is
   either filed or dismissed.
3. **Nothing audits the ledger itself.** No orphan / broken-link / duplicate / stale
   check exists; the only cluster detection is HQ reading 146 titles by eye. The
   survey found every mature practice (Karpathy's LLM Wiki, obsidian-mind, A-MEM) runs
   a lint and finds neighbours at intake.

And the structural one: the KB's code is 3.5k lines spread over eleven files in
`internal/hq` (`knowledge*.go`, `capturecmd.go`, `distill.go`, `mine.go`), with the
seed's topic files hard-coded in `hq.go` and the API shapes in `knowledgeapi.go`. The
next iteration of any of this touches the supervisor package as a whole.

## What Changes

**A. One module.** A new leaf package `internal/knowledge` owns everything about the
ledger: the record format and its migration, the vocabulary, provenance, audience,
the candidate pool, promote/land/withdraw, lint, neighbours, rendering, distribution,
and the index/entry shapes the CLI, serve and the two screens read. `internal/hq` keeps
only what is supervision: the distill and mine sensors, the CLI verb dispatch (thin
shims), the playbook. `internal/mine` stays a leaf and does not import `knowledge`.
Phase 1 is a PURE MOVE with no behaviour change, the way `decompose-app-package` was.

**B. Three orthogonal axes on every entry** (design D2–D4):
- `kind` — what it is: `facts` · `howto` · `pitfalls` · `judgment` · `decisions`
  (CoALA's semantic/procedural split, one level finer). `topic` stays as a free tag
  list, so HQ-declared topics keep working.
- `provenance` — where it came from: `correction` · `recurrence` · `mined` · `capture`
  · `self`, with a count. `corrections` stops being a topic; the count is ACE's
  helpful/harmful counter and the recurrence feed-back signal.
- `audience` — who must know it: `hq` · `machine` · `repo` · `everyone`. This replaces
  the free-text `--target` of promote. In the UI the four are one word each:
  中控 · 本机 · 仓库 · 全体.

Plus a status `hypothesis` for a filed lead not yet verified, between "candidate" and
"live".

**C. Distribution, per audience** (D5–D6):
- `hq` → LOCAL.md (as today).
- `machine` → ONE canonical file gtmux renders (`~/.config/gtmux/knowledge/machine.md`),
  fanned out as a **managed, sentinel-delimited block** into each supported agent's
  global instruction file (Claude Code, Codex, opencode, Kimi Code — the same installer
  shape hooks already use). The block carries an INDEX (one line per entry + the path
  to the canonical file), never the full text: Skills-style progressive disclosure, so
  it cannot grow into every session's context. `gtmux knowledge sync` refreshes;
  `gtmux doctor` gets a "knowledge sync" row per agent.
- `repo` → a managed block in that repository's `AGENTS.md` (or `CLAUDE.md` when that is
  the only one present), full text since it is small and local; the commit is left to
  the person, and the UI says so.
- `everyone` → "feedback to gtmux": a prefilled GitHub issue (or copy-to-clipboard),
  landed with the issue URL. Briefs with this audience do NOT count toward the two-week
  overdue floor — a product change is not a user's debt.

**D. Verbs.** `withdraw <id> --why` returns a promoted entry to live (the entry was right,
the promotion was not — neither `retire` nor a fake `land` says that). `promote` takes
`--for <audience>` instead of `--target`. `lint` runs the ledger checks; `neighbours <id
| --capture <key>>` lists the closest entries by kind + keyword overlap; `capture --list`
groups the pool by neighbourhood so a family of leads is one `add --capture k1,k2,…`.

**E. Surfaces.** The menu-bar window and the phone show kind / provenance / audience on
an entry, a "write it in" action for `hq` / `machine` / `repo` (preview, then append,
then land) and "feedback to gtmux" for `everyone`; the spool view groups by neighbour.
`GET /api/hq/knowledge` carries the new fields; `POST …/act` gains `carry` and
`withdraw`.

**F. Migration.** Ledger records gain fields additively; a read of an old record maps
`environment`/`accounts` → `facts`, `workflows` → `howto`, `best-practices` →
`judgment`, `pitfalls` → `pitfalls`, and `corrections` → `judgment` with
`provenance: correction` and a lint flag "kind assumed at migration — confirm". Old
topic files keep rendering until HQ confirms. Nothing is rewritten in place.

## What Does NOT Change

- The ledger stays append-only with provenance; `add` / `supersede` / `retire` /
  `dismiss` keep their meaning; render stays deterministic.
- distill stays the single quality gate and NEVER rewrites a topic file wholesale (ACE's
  collapse mode is a stated constraint, not a hope).
- The candidate pool format (fields are additive).
- `internal/mine` and its ledger.

## Privacy boundary

Distribution writes only into the user's own files, only inside sentinel blocks, never
outside them. Tests use synthetic ledgers and synthetic agent instruction files in a temp
dir; nothing from a real KB enters the repo.
