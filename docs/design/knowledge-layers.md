# Three axes and four readers of knowledge: kind · provenance · audience

The chief of staff carries three "places where rules and facts get written down", and they all look like knowledge but belong to completely different owners.
Blur them and you get this illusion: a general improvement that everyone should have received stays in one machine's private ledger
(on 2026-09-05 the commander asked exactly that: 「我以为知识库是个性化的本地用户信息，这个看起来是通用的优化建议」, "I thought the knowledge base was personal local user info; this looks like a general optimization suggestion").

This document answers four questions: where knowledge lives, what one entry is, where it came from, and who must know it.
The same ground for a READER rather than a maintainer is [docs/knowledge.md](../knowledge.md).
The first is the "three layers"; the other three are the "three axes" every entry carries (openspec change `hq-knowledge-engine`;
the research behind it is in `knowledge-engineering-research.md`).

## What "the knowledge base" is, and what it is not

The knowledge base is one folder, `~/.config/gtmux/hq/knowledge/` (the commander asked for this
pinned down on 2026-09-14: 「这里的知识库具体指什么」). It holds:

- the ledger, `.ledger.jsonl`: append-only, written only through `gtmux knowledge add /
  supersede / retire / promote / land / …`, every change in the audit stream;
- the topic files gtmux renders from it (`pitfalls.md`, `best-practices.md`, `accounts.md`, …),
  overwritten on every render, never hand-edited;
- `promotions/`, the take-away briefs of entries promoted and not yet landed;
- `tools/`, HQ's scripts, each named by one `howto` entry (kb-tools-in-knowledge).

`~/.config/gtmux/knowledge/machine.md` is NOT part of it, and the two folders being one
letter apart is the reason this paragraph exists (asked twice, 2026-09-18). That file is
the OUTBOX: the entries whose audience is `machine`, rendered for distribution. Naming them
store and outbox is what failed to land, twice, with the person who asked: the difference
that explains itself is the READER. No other agent can see HQ's folder, every agent reads
its own global instruction file, and this file is the only route between the two. The store
is written by the verbs and read by HQ; the outbox is generated, and deleting it costs
nothing because the next `sync` writes it again. The whole layout on disk:

```
~/.config/gtmux/
├── hq/                        HQ's home — everything below is its records (档案)
│   ├── AGENTS.md              the charter, gtmux-owned, regenerated on update
│   ├── LOCAL.md               the operator's rules, seeded once, never overwritten
│   ├── notes/board.md         HQ's current posture, not knowledge
│   └── knowledge/             THE KNOWLEDGE BASE
│       ├── .ledger.jsonl      the authority: append-only, every change
│       ├── <topic>.md         rendered from the ledger, never hand-edited
│       ├── promotions/        briefs for entries promoted, not yet landed
│       └── tools/             HQ's scripts, each named by one entry
└── knowledge/machine.md       GENERATED: the `machine`-audience entries, for every agent
```

An entry is one lesson with a kind, a provenance count, a lifecycle and, once promoted, an
audience. It is loaded on demand, echoed to a worker at dispatch or looked up by HQ, and is
out of context otherwise.

`LOCAL.md` (the commander's standing rules, in context every turn), `AGENTS.md` (gtmux's
charter) and `notes/board.md` (HQ's current posture) are outside it. Those four things
together, the whole home folder, are **HQ's records** (档案): what `gtmux hq --export` packs,
`--import` restores and `--records` measures. The bundle was called "memory" until
2026-09-14; the word was retired because it read as a name for the knowledge base and
collides with what "memory" means for an agent's context.

## Three layers: where knowledge lives

| | Factory charter `AGENTS.md` | Your rules `LOCAL.md` | This machine's ledger `knowledge/` |
|---|---|---|---|
| Owner | the gtmux product | this operator | this machine's chief of staff |
| Who writes it | code: `hqInstructions` in `internal/hq/hq.go` + `playbook_zh.go` | you, by hand; gtmux appends only when you tell it to "write it in" | the chief of staff, as it works |
| How it updates | change the code + bump `hqPlaybookVersion` → shipped with `gtmux update`, regenerated | you edit it; **gtmux never overwrites it** (seeded once) | `gtmux knowledge add/supersede/retire/…`, an append-only ledger |
| When it applies | in context every session | in context every session (the `@LOCAL.md` import at the end of `AGENTS.md`) | on demand: echoed to a worker at dispatch by repo name/keyword; otherwise the chief of staff looks it up |
| Shape | one managed document, not hand-editable | one document of yours | a ledger with provenance (`internal/knowledge`; supersede, retire, audited) |

**A `[[link]]` follows the entry that replaced its target.** `supersede` gives the rewritten
lesson a new id, so a body that linked to the old one goes on naming something that is no
longer there. Following such a link returns nothing, and returns it quietly: no error, no
empty result, so the reader takes the silence for "I have already read this". On this
machine 38 of 524 entries carried one, and a single lesson was referenced by six entries
and reachable from none of them until somebody noticed it was missing from the base. The
successor was never lost — every supersede records it and the linter has been computing it
all along in order to SAY the link was stale. Reporting it is not the same as using it, so
the fold now resolves each link to the entry that is actually there, following a chain to
its end. A name nothing replaced is left as written: an unknown one is a placeholder for an
entry not yet written.

The import order is deliberate: the body of `AGENTS.md` comes first and `@LOCAL.md` last, so your rules extend and override the factory charter.

Scripts are ledger entries with a file attached (kb-tools-in-knowledge, 2026-09-14). A script HQ
writes lives in `knowledge/tools/`, and one `howto` entry names its path and says when to run it;
the ledger is the only index of what HQ can do. On the design machine a top-level `tools/` had
grown its own `README.md` index while the ledger pointed at the same scripts thirty times, so one
question had two indexes, and the README was the one nobody read before dispatching. `gtmux knowledge lint`
reports a script no entry names (`orphan-tool`) and an entry naming a script that is not there
(`broken-tool`). The home's top level is those four things and nothing else: a brief goes in
`notes/` as a note, and there is no `designs/` folder.

## Three axes: the three slots on every entry

Every ledger entry occupies one slot on each of three orthogonal axes. Both the chief of staff and a human can answer "what is this, where did it come from, who is it for".

**Kind: what it is.** One level finer than CoALA's semantic / procedural memory split:

| kind | Meaning | Example |
|---|---|---|
| `facts` | how this machine, this account, this world is | the office network TLS-resets wrangler |
| `howto` | how something is done | release: push the tag, wait for the app job, install |
| `pitfalls` | don't do this | don't target a udid when installing; it hangs on a locked phone |
| `judgment` | how to judge in which situation | the context window ceiling is what it actually reached, not the model name |
| `decisions` | why A was chosen over B | distribution carries only an index, not full text, because the block enters every session's context |

`topic` remains the id prefix and a free tag (`--tags`); the topics the chief of staff declares itself are unchanged. Entries written before the axes existed
get a kind mapped from a fixed table at read time and are marked "presumed" (rendered with a `?`); `gtmux knowledge kind <id> <kind>` confirms it;
the ledger file itself is never rewritten, and the first write in the new format backs the old file up as `.ledger.jsonl.bak-v1`.

**Provenance: where it came from, and how many times.** `correction` (the commander corrected it) · `recurrence` (the same trap hit again) ·
`mined` (dug out by session mining) · `capture` (a worker jotted it down) · `self` (the chief of staff observed it), with a count.
`gtmux knowledge hit <id>` raises the count; the miner records one automatically when it recognizes an error signature already in the ledger. A lesson already
in the ledger whose count keeps rising says the carrier is not being read (the lesson was recorded; it was never delivered): this is ACE's "helpful / harmful count", and the signal that closes the learning loop.
`corrections` used to be a topic, which classified by "who told me"; now it is this slot.

**Audience: who must know it.** This axis is filled only at promotion, four words, the same four shown on both screens:

| `--for` | Word | Who reads it | Where it lands |
|---|---|---|---|
| `hq` | HQ | only the chief of staff itself | `LOCAL.md`, gtmux appends a section |
| `machine` | this machine | every agent on this machine | one master copy at `~/.config/gtmux/knowledge/machine.md`, plus an index block in each agent's global instruction file |
| `repo:<path>` | repo | agents working in that repo | a block in that repo's `AGENTS.md` (`CLAUDE.md` when that is all there is), not committed |
| `everyone` | everyone | all gtmux users | the gtmux product: a pre-filled GitHub issue |

If "who" cannot be determined, it should not be promoted; it stays in the ledger.

There is also a `hypothesis` state: a lead from mining or self-report not yet confirmed; `add --hypothesis` puts it in its own section,
undistributed, and `confirm <id>` makes it regular.

## Two languages: one entry, both halves

Every entry records the language it was written in (`lang`) and may carry the other as an
alternate half (title and body), written by HQ for that language's reader; gtmux never
translates, since it calls no model. Readers get their language by one rule on every
surface: the source when it matches, else the alternate, else the source with a tag saying
which language it is. Files on this machine (topic files, `machine.md`, the agents' index
blocks, repo blocks) render in the base's majority language; the syncing process's own
language is ignored, because launchd leaves `GTMUX_LANG` unset and a block that flipped per
renderer would churn every agent's file. The `everyone` brief and its issue prefill go out
in English. A missing half is a `monolingual` lint count HQ works down in batches; it never
blocks a capture. Change `kb-bilingual`, 2026-09-14.

## Sensitive entries: the commander's own detail

The base may hold the commander's own detail: an account, a personal fact, a credential they
chose to keep here (2026-09-14: 「KB 可以记录敏感信息，但是要求用户确认好」). Two rules make it
safe enough on a machine that is theirs. **Ask first, and record the asking**: HQ shows the
exact title and body, gets an explicit yes in that turn, and writes with `--sensitive
--confirmed "<their words>"` (or marks an existing entry with `sensitive <id> --confirmed …`);
the ledger refuses a sensitive write with no words. **It stays here**: `promote` accepts only
`hq` for it, `machine.md` and the repo blocks never render it, and every surface shows a lock.
`lint` reports `unmarked-sensitive` for an entry that reads like a credential without the
mark, which means one written without asking. Other people's secrets are still out of scope:
the ledger may hold a pointer to where they live, never the secret itself. Nothing is
encrypted on disk; the copy that travels is the export, and that is what the passphrase lock
(hq-export-passphrase) covers.

## The exit: promote → land, or withdraw

The ledger is private to the machine, but it grows entries bigger than the machine. The exit is a fixed sequence of commands, so nobody has to remember to do it:

1. The chief of staff judges an entry big enough → `gtmux knowledge promote <id> --why … --for <hq|machine|repo:<path>|everyone>`
2. gtmux writes a take-away brief under `knowledge/promotions/`: the lesson verbatim, why, the audience, full provenance, and the exit for that audience
3. Land it. For the first three audiences gtmux moves it for you: `gtmux knowledge land <id>` writes it where that audience reads and closes it.
   `everyone` is a human opening an issue (the brief has a pre-filled link; on both screens it is "Feedback to gtmux ↗"), then `land <id> --ref <issue link>`.
   Moving it yourself is fine too: `land <id> --ref <where>`
4. The chief of staff judged wrong and the entry is not worth moving: `gtmux knowledge withdraw <id> --why …`, and the entry returns to "alive".
   `retire` would be wrong here (the entry is not wrong), and so would an invented provenance
5. `gtmux doctor` watches the queue: anything unmoved for about two weeks is flagged; `everyone` is not counted as overdue, since the product's business should not put a red line on the user

## Distribution: how machine knowledge reaches every agent

The `machine` master copy is rendered by gtmux, which then maintains a managed block, with sentinels and a hash, in every supported agent's global instruction file:
Claude Code `~/.claude/CLAUDE.md`, Codex `$CODEX_HOME/AGENTS.md`, opencode `~/.config/opencode/AGENTS.md`,
Kimi Code `$KIMI_CODE_HOME/AGENTS.md` (paths recorded in the agent registry `internal/agents`).

The block carries only an index (one sentence per entry, plus the master path) and never the full text: the block enters every session's context, so the index cannot grow long,
and all four agents can read files (progressive disclosure, as in Skills). Content outside the block is never touched; a block edited by hand makes `sync` refuse to overwrite,
and only `--force` writes. `gtmux knowledge sync` refreshes, `carriers` shows each agent's state, doctor has a "knowledge distribution" row, and `--fix` fills in what is missing or stale.

The `repo` block carries the full text (small; only that repo's agents read it); gtmux writes the file without committing, the commit is yours.

## Beyond the three layers: the candidate pool and two gates

`gtmux capture "<one lesson> @<topic>"` is the cheapest entry point: a worker jots one line, which lands in the candidate pool at
`knowledge/.pending-distill.jsonl`, and the ledger is untouched. The pool's second feeder is the session miner
(`gtmux knowledge mine`, which serve runs once a day): no model involved, it subtracts everything the machine itself wrote from each agent's session logs
and drops "the correction a human typed right after an agent reply" and "an error recurring across sessions" into the same pool.

The chief of staff's distill is the **only quality gate**: merge candidates about the same thing into one entry (`knowledge add --capture k1,k2,…`;
`capture --list` already groups them by family), or reject with a reason (`dismiss --why …`; a rejection leaves a trace too). Distill is incremental only,
never rewrites a whole topic file, and `supersede` keeps the old text (`show <old id>` still reads it); whole-file rewrites and lost old text are the two collapse paths ACE's experiments demonstrated.

The second gate audits the ledger itself: `gtmux knowledge lint` reports orphans, broken and stale links, likely duplicates, overdue promotions, unconfirmed kinds, and `ai-voice`: an entry that reads like a machine wrote it. That last one holds the mechanical half of the 2026-09-16 pass that rewrote 485 of 505 entries into plain language, and it borrows the humanizer skill's ranking, which is the whole of its design: chat residue, a decorative `⇒` and the in-house coinages are things this base has already decided against, so one sighting is a defect, while a dash, a bold run and a "not X, but Y" each have honest uses and count only in company. It reads prose only, never code, tables or quoted spans, and it never judges whether a contrast is earned — that reading stays with whoever rewrites the entry. It reports, never edits;
its one-line summary rides the self-check knock to the chief of staff. `neighbours` finds the closest entries, and `add` lists three before writing:
an entry about the same thing is a `supersede` of the existing one.

## Authoritative files

- The ledger with all its verbs, rendering, distribution, lint: `internal/knowledge/` (a leaf package; `hq` keeps only supervision)
- The miner: `internal/mine/`
- Design decisions and the rejected alternatives: `openspec/changes/archive/2026-09-12-hq-knowledge-engine/design.md` (D1 to D10)
- Research: `knowledge-engineering-research.md`
