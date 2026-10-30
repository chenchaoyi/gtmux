# HQ knowledge base

**English** · [中文](knowledge.zh.md)

<a id="what-hq-remembers"></a>

HQ is the supervisor session started by `gtmux hq`. As it coordinates tasks, it records reusable procedures, environment issues and user corrections in a knowledge base on your host. Selected lessons can be promoted for use across the host and synced into supported agents’ instruction indexes for new sessions to consult.

This page covers storage, how entries are used and distributed, and what you can edit. The contents stay on the host unless you send them elsewhere.

## Read and use knowledge in the interface

Every ordinary entry shows **HQ reference** by default: HQ can look it up and refer to it when assigning work. The entry has no recorded standing-instruction use.
Keep occasional background information here. Use **HQ standing rules** for recurring delegation, review and escalation agreements.

Mac and phone lists and details show the recorded use and processing state without opening a dialog:

| Use | Suitable knowledge | Next step |
|---|---|---|
| HQ reference | Background and occasional reference | None |
| HQ standing rules | Delegation, review and when HQ should ask you to decide | Write to HQ instructions (`LOCAL.md`) |
| Agents on this computer | Shared environment facts and precautions | Sync local agents' instruction entries |
| Agents in one project | Project build, test and coding conventions | Write project instructions; files are not committed automatically |
| gtmux product feedback | A broadly useful product improvement | Review and submit the prefilled issue, then record its URL |

On Mac, **Choose how to use** saves a plan and reason. Complete **Write instructions** or product feedback from the same entry.
**Write pending** and **Application recorded** describe ledger progress, not proof that agents loaded the content; running sessions may need to reload.
The machine master text is rendered from the ledger and may already be reachable through existing entries, so pending is not a filesystem access guarantee.
Older processed records without a recognized use show **Use not recorded** instead of guessing that all agents received them.
Cancelling a plan retains the knowledge and does not roll back instruction files already written. A missing audience record does not prove that an instruction file never received a copy.

Categories, provenance and built-in topics use reader-facing names; inferred categories say **unconfirmed category**. Custom topics retain their names.
Identifiers, reasons and recorded destinations are in **Record details**. **Backup and migration** covers HQ instructions, task records and knowledge together; it is not an entry's scope.
Use describes context-loading intent rather than file permissions between processes running as the same user.

## Where it lives

Everything HQ owns is under `~/.config/gtmux/hq/`, and one generated file sits outside it:

| Path | What it is | Who writes it |
|---|---|---|
| `hq/knowledge/.ledger.jsonl` | the knowledge base itself: every entry, every change, append-only | HQ, through `gtmux knowledge …` |
| `hq/knowledge/*.md` | the same entries rendered by topic, for reading | gtmux, overwritten on every render |
| `hq/knowledge/promotions/` | the take-away brief of an entry waiting to be carried somewhere | gtmux |
| `hq/knowledge/tools/` | scripts HQ wrote for itself, each named by one entry | HQ |
| `hq/AGENTS.md` | HQ's charter: how it works, shipped with gtmux | gtmux, replaced on update |
| `hq/LOCAL.md` | your own standing rules | you; ordinary updates preserve it. HQ-targeted landing can append a section, and an explicit restore or migration can replace it |
| `hq/notes/board.md` | HQ's current session progress and decisions, separate from knowledge | HQ |
| `knowledge/machine.md` | the handful of lessons every agent on this machine must know | gtmux, generated |

Two of those folders are called `knowledge`, but they serve different readers.
`hq/knowledge/` holds HQ's complete ledger; it is not automatically loaded into ordinary
project sessions. That is a distribution boundary, not filesystem isolation: an agent
with access to those files can read them when directed to them.

The normal machine-wide distribution path uses supported agents' global instructions,
such as `~/.claude/CLAUDE.md` for Claude Code. The generated index points at `machine.md`
for the selected lessons, rather than loading HQ's whole base into every session.

For example, an interactive `rm` alias can cause a non-interactive agent to skip a deletion when confirmation is unavailable. Recording that issue in HQ’s knowledge base does not automatically inform new sessions. Syncing the lesson to `machine.md` and the agents’ instruction indexes makes it available at startup to sessions using those instructions.

Only a small set of lessons that apply across agents belong in the shared instructions. HQ can look up the rest when needed, without adding the full knowledge base to every session’s context. If `machine.md` is deleted, the next sync regenerates it.

## Three layers, and which one is yours

Rules reach an agent from three places, and they are not interchangeable:

| | HQ's charter `AGENTS.md` | Your rules `LOCAL.md` | This machine's base `knowledge/` |
|---|---|---|---|
| Whose it is | gtmux's | yours | this machine's |
| Who writes it | the product, shipped with each release | you | HQ, as it works |
| When it applies | every session | every session | when it is relevant, or when HQ looks it up |
| Can you edit it | no, it is regenerated | yes, this is the place for that | through the commands below, not by hand |

`LOCAL.md` is imported at the end of the charter, so what you write there extends and
overrides what gtmux ships. If you want HQ to always do something, write it there. If you
want it to remember something it worked out, that is the base.

## How a lesson travels

```
something happens  →  HQ records an entry  →  (optional) promote  →  land
```

HQ writes an entry the moment it learns something durable: what happened,
how to tell it is happening again, what to do. Any agent can drop a one-line candidate
with `gtmux capture "<lesson> @<topic>"`; HQ decides what becomes an entry. Once a day
gtmux also reads the agents' session logs, with no model involved, and queues the places
where you corrected someone or a tool failed repeatedly.

Most entries stay where they are. An entry stops being about this machine
when it is about you, about every agent here, about one repository, or about gtmux
itself, and HQ promotes it, saying who must know:

| Audience | Who reads it | Where it lands |
|---|---|---|
| HQ standing rules | HQ instructions | a section appended to your `LOCAL.md` |
| this machine | every agent on this Mac | `knowledge/machine.md`, plus a short block in each agent's global instruction file |
| a repository | agents working in that repo | a block in that repo's `AGENTS.md` (or existing `CLAUDE.md` when no `AGENTS.md` exists), written but never committed |
| product feedback | feedback you review and submit to gtmux | a pre-filled GitHub issue for you to open |

Promotion writes a brief. `land` without `--ref` carries it to the selected local
audience; `land --ref` records where you have already carried it. The common case is
"this machine": your Claude Code, Codex, OpenCode and Kimi Code each get a block naming
the lessons and pointing at the full text, so a fresh session knows them without anyone
pasting anything. `gtmux knowledge carriers` shows each agent's file
and whether it is current; `gtmux doctor --fix` repairs a stale one.

When an entry no longer applies, use `retire` with a reason. The ledger keeps the change.

## When machine instructions are synced

Landing an entry for "this machine" generates `~/.config/gtmux/knowledge/machine.md`
and refreshes the short index block in the global instruction files for Claude Code,
Codex, opencode, and Kimi Code. Ordinary entries stay in HQ's knowledge base; they do
not automatically enter every agent's instructions. Installing gtmux or running
`gtmux update` does not perform this sync. After an upgrade, run
`gtmux knowledge sync` if the blocks are stale, or use `gtmux doctor --fix` to check
and approve a repair. `gtmux knowledge carriers` lists the supported files and status.
The new index block also includes a short `gtmux relay` entry point supplied by gtmux;
it is not a knowledge entry promoted by HQ.

Sync changes only the text between the `gtmux:knowledge` markers and preserves text
outside the block. If someone edited the block by hand, sync refuses to overwrite it
unless you review it and use `gtmux knowledge sync --force`. Agents read their global
instructions when a new session starts; a running session does not reload them after
a sync. Other agent types currently have no global knowledge distribution channel.

## Reading it and changing it

On the phone and iPad, open HQ and tap **Knowledge**.

![HQ on iPad: the Knowledge card is at the top, between Board and Usage](guides/img/hq-supervisor-ipad-en.jpg)

The knowledge list shows what is waiting on you first. Open an entry to read it.

![The knowledge list on iPad, with an entry open beside it](guides/img/hq-supervisor-knowledge-en.jpg)

In the menu bar, expand the HQ card and click its **KNOWLEDGE** row to open the same list.

![The expanded menu-bar HQ card with its KNOWLEDGE row](guides/img/knowledge-menubar-en.png)

From the terminal:

```sh
gtmux knowledge list                 # every live entry
gtmux knowledge list --topic pitfalls
gtmux knowledge show <id>            # one entry in full
gtmux knowledge lint                 # issues and clearly labeled review/info hints; never edits
gtmux knowledge carriers             # which agents have the machine block, and is it current
gtmux knowledge promotions           # what is promoted and waiting to be carried
```

Changing it is HQ's job and those verbs are its tools. Three of them you may want
yourself:

```sh
gtmux knowledge retire <id> --why "the office network was fixed"
gtmux knowledge sensitive <id> --confirmed "<your own words>"   # keep it on this Mac
gtmux knowledge sync                                            # push the machine block out again
```

The everyday way to change what HQ knows is to tell HQ, in its own session, and let it
write the entry. That keeps the reason, the provenance and the language halves intact,
which is the part a hand edit loses.

## Two languages

Every entry is written twice, in English and Chinese, each half written for that
language's reader rather than translated. Every surface shows you the half in your
language and marks the entry when only the other half exists. A missing half is a
counted backlog, never a reason not to record something.

## Your own information

You can ask HQ to keep something personal: an account you use, a path that is yours, a
preference. Those entries are marked sensitive, and gtmux treats the mark as a hard rule:
they are never rendered into `machine.md`, never written into a repository block, never
part of an exported brief, and every surface shows them with a lock. HQ only marks an
entry sensitive after you say so, in your words, and it records your words.

gtmux does not upload the knowledge base automatically. `gtmux hq --export` creates an
encrypted backup you can save or transfer; `gtmux hq --memory` reports its size. In the
menu bar, **HQ → Knowledge → Import…** offers two separate actions:

- **Restore HQ backup** restores the complete HQ home, including the old board, and
  retains the current home as a separate backup.
- **Move from another Mac** selects live knowledge and its revision history, personal
  requirements, or tool attachments for staging only. Nothing starts checked. The old
  board, sessions, pending leads, built-in rules and connection credentials do not move.

Review knowledge before applying it; sensitive histories need a separate opt-in and old
audience decisions are reset. Compare personal requirements side by side; keeping current
is the default, replacement is a separate decision. **Continue staged migration** resumes
a review after closing. Exit HQ before apply or restore; idle is still running.
The CLI equivalent is `gtmux hq migrate --help`; `--import` remains a complete restore.
See [Moving HQ](design/hq-move-between-macs.md) for exact rules and limits.

## If you want to read further

[docs/cli.md](cli.md) has every command and flag. The design behind it, including why the
layers are separate and what happens at each gate, is in
[docs/design/knowledge-layers.md](design/knowledge-layers.md).

## Sources and processing receipts

Each candidate now has an observation ID and payload digest; its family key still groups
related lines. Acceptance or dismissal commits all retained source text, context and
available metadata to the ledger before excluding those IDs from the pending view.
Validation or write failure keeps the work pending. New observations with the same key
remain pending. Dismissals retain their reason without becoming knowledge entries.

Use `gtmux knowledge show <id> --json` for entry sources and
`gtmux knowledge receipts --capture <key> --json` for committed acceptances/dismissals.
Supersede retains source lineage; receipts survive retirement. Unknown source positions
stay unknown, and sources already discarded by earlier versions cannot be recovered.
The digest detects changed payload bytes; it is not a truth score. Raw context stays out
of indexes, generated rules, agent instruction blocks and public promotion briefs.

Failures are reported directly and correlated by `op_id`, `outcome` and `phase` in
events/diagnostics when those logs are writable. If an error says the operation committed
but rendering failed, inspect its receipt and run `gtmux knowledge render`; do not accept
the same observations again. Writes share a process lock and replace the file only after
a complete temporary copy is ready, using space proportional to that file. Newly written
ledger/archive files are private. Use the current CLI for queue processing: older versions
may truncate the source archive under their former queue semantics.
