# Three axes and four readers of knowledge: kind · provenance · audience

The chief of staff carries three "places where rules and facts get written down", and they **all look like knowledge but belong to completely different owners**.
Blur them and you get this illusion: a general improvement that everyone should have received stays in one machine's private ledger
(on 2026-09-05 the commander asked exactly that: 「我以为知识库是个性化的本地用户信息，这个看起来是通用的优化建议」, "I thought the knowledge base was personal local user info; this looks like a general optimization suggestion").

This document answers four questions: **where knowledge lives, what one entry is, where it came from, and who must know it.**
The first is the "three layers"; the other three are the "three axes" every entry carries (openspec change `hq-knowledge-engine`;
the research behind it is in `knowledge-engineering-research.md`).

## Three layers: where knowledge lives

| | Factory charter `AGENTS.md` | Your rules `LOCAL.md` | This machine's ledger `knowledge/` |
|---|---|---|---|
| Owner | **the gtmux product** | **this operator** | **this machine's chief of staff** |
| Who writes it | code: `hqInstructions` in `internal/hq/hq.go` + `playbook_zh.go` | you, by hand; gtmux appends only when you tell it to "write it in" | the chief of staff, as it works |
| How it updates | change the code + bump `hqPlaybookVersion` → shipped with `gtmux update`, regenerated | you edit it; **gtmux never overwrites it** (seeded once) | `gtmux knowledge add/supersede/retire/…`, an append-only ledger |
| When it applies | **in context every session** | **in context every session** (the `@LOCAL.md` import at the end of `AGENTS.md`) | **on demand**: echoed to a worker at dispatch by repo name/keyword; otherwise the chief of staff looks it up |
| Shape | one managed document, not hand-editable | one document of yours | a ledger with provenance (`internal/knowledge`; supersede, retire, audited) |

**The import order is deliberate**: the body of `AGENTS.md` comes first and `@LOCAL.md` last, so your rules **extend and override** the factory charter.

## Three axes: the three slots on every entry

Every ledger entry occupies one slot on each of three orthogonal axes. Both the chief of staff and a human can answer "what is this, where did it come from, who is it for".

**Kind — what it is.** One level finer than CoALA's semantic / procedural memory split:

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

**Provenance — where it came from, and how many times.** `correction` (the commander corrected it) · `recurrence` (the same trap hit again) ·
`mined` (dug out by session mining) · `capture` (a worker jotted it down) · `self` (the chief of staff observed it), with a count.
`gtmux knowledge hit <id>` raises the count; the miner records one automatically when it recognizes an error signature already in the ledger. **A lesson already
in the ledger whose count keeps rising says the carrier is not being read, not that it was forgotten**: this is ACE's "helpful / harmful count", and the signal that closes the learning loop.
`corrections` used to be a topic, which classified by "who told me"; now it is this slot.

**Audience — who must know it.** This axis is filled only at promotion, four words, the same four shown on both screens:

| `--for` | Word | Who reads it | Where it lands |
|---|---|---|---|
| `hq` | HQ | only the chief of staff itself | `LOCAL.md`, gtmux appends a section |
| `machine` | this machine | every agent on this machine | one master copy at `~/.config/gtmux/knowledge/machine.md`, plus an index block in each agent's global instruction file |
| `repo:<path>` | repo | agents working in that repo | a block in that repo's `AGENTS.md` (`CLAUDE.md` when that is all there is), **not committed** |
| `everyone` | everyone | all gtmux users | the gtmux product: a pre-filled GitHub issue |

If "who" cannot be determined, it should not be promoted; it stays in the ledger.

There is also a `hypothesis` state: a lead from mining or self-report not yet confirmed; `add --hypothesis` puts it in its own section,
undistributed, and `confirm <id>` makes it regular.

## Two languages: one entry, both halves

Every entry records the language it was written in (`lang`) and may carry the other as an
alternate half (title and body), written by HQ for that language's reader — never
translated by gtmux, which calls no model. Readers get their language by one rule on every
surface: the source when it matches, else the alternate, else the source with a tag saying
which language it is. Files on this machine (topic files, `machine.md`, the agents' index
blocks, repo blocks) render in the base's majority language, not the syncing process's
(launchd leaves `GTMUX_LANG` unset, and a block that flipped per renderer would churn every
agent's file); the `everyone` brief and its issue prefill go out in English. A missing half
is a `monolingual` lint count HQ works down in batches; it never blocks a capture. Change
`kb-bilingual`, 2026-09-14.

## The exit: promote → land, or withdraw

The ledger is private to the machine, but it grows entries bigger than the machine. The exit is mechanical, not memory-dependent:

1. The chief of staff judges an entry big enough → `gtmux knowledge promote <id> --why … --for <hq|machine|repo:<path>|everyone>`
2. gtmux writes a **take-away brief** under `knowledge/promotions/`: the lesson verbatim, why, the audience, full provenance, and the exit for that audience
3. **Land it**. For the first three audiences gtmux moves it for you: `gtmux knowledge land <id>` writes it where that audience reads and closes it.
   `everyone` is a human opening an issue (the brief has a pre-filled link; on both screens it is "Feedback to gtmux ↗"), then `land <id> --ref <issue link>`.
   Moving it yourself is fine too: `land <id> --ref <where>`
4. The chief of staff judged wrong and the entry is not worth moving: `gtmux knowledge withdraw <id> --why …`, and the entry returns to "alive".
   Not `retire` (the entry is not wrong), and not an invented provenance
5. `gtmux doctor` watches the queue: anything unmoved for about two weeks is flagged; `everyone` is not counted as overdue, since the product's business should not put a red line on the user

## Distribution: how machine knowledge reaches every agent

The `machine` master copy is rendered by gtmux, which then maintains a managed block, with sentinels and a hash, in every supported agent's global instruction file:
Claude Code `~/.claude/CLAUDE.md`, Codex `$CODEX_HOME/AGENTS.md`, opencode `~/.config/opencode/AGENTS.md`,
Kimi Code `$KIMI_CODE_HOME/AGENTS.md` (paths recorded in the agent registry `internal/agents`).

The block **carries only an index** (one sentence per entry, plus the master path), never the full text: the block enters every session's context, so the index cannot grow long,
and all four agents can read files (progressive disclosure, as in Skills). Content outside the block is never touched; a block edited by hand makes `sync` refuse to overwrite,
and only `--force` writes. `gtmux knowledge sync` refreshes, `carriers` shows each agent's state, doctor has a "knowledge distribution" row, and `--fix` fills in what is missing or stale.

The `repo` block carries the full text (small; only that repo's agents read it); gtmux writes the file without committing, the commit is yours.

## Beyond the three layers: the candidate pool and two gates

`gtmux capture "<one lesson> @<topic>"` is the **cheapest entry point**: a worker jots one line, which lands in the candidate pool at
`knowledge/.pending-distill.jsonl`, **not** in the ledger. The pool's second feeder is not a human but the **session miner**
(`gtmux knowledge mine`, which serve runs once a day): no model involved, it subtracts everything the machine itself wrote from each agent's session logs
and drops "the correction a human typed right after an agent reply" and "an error recurring across sessions" into the same pool.

The chief of staff's distill is the **only quality gate**: merge candidates about the same thing into one entry (`knowledge add --capture k1,k2,…`;
`capture --list` already groups them by family), or reject with a reason (`dismiss --why …`; a rejection leaves a trace too). Distill is incremental only,
never rewrites a whole topic file, and `supersede` keeps the old text (`show <old id>` still reads it). Those are the two collapse paths ACE's experiments demonstrated.

The second gate audits the ledger itself: `gtmux knowledge lint` reports orphans, broken and stale links, likely duplicates, overdue promotions and unconfirmed kinds; it reports, never edits;
its one-line summary rides the self-check knock to the chief of staff. `neighbours` finds the closest entries, and `add` lists three before writing:
the same thing should be a `supersede`, not another entry.

## Authoritative files

- The ledger with all its verbs, rendering, distribution, lint: `internal/knowledge/` (a leaf package; `hq` keeps only supervision)
- The miner: `internal/mine/`
- Design decisions and the rejected alternatives: `openspec/changes/archive/2026-09-12-hq-knowledge-engine/design.md` (D1–D10)
- Research: `knowledge-engineering-research.md`
