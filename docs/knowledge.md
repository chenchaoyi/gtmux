# What HQ remembers

**English** · [中文](knowledge.zh.md)

HQ is the supervisor session `gtmux hq` starts. As it watches your agents it learns
things: that this office network resets TLS handshakes, that a command in this repo needs
a flag nobody remembers, that you asked twice for links to be plain URLs. Those go into a
knowledge base on your Mac, and the next time an agent starts work here, the ones that
apply are already in front of it.

This page covers where that base sits, how a lesson gets from "HQ noticed it" to "every
agent on this machine reads it", and what you can change yourself. Nothing here leaves
your machine unless you send it somewhere.

## Where it lives

Everything HQ owns is under `~/.config/gtmux/hq/`, and one generated file sits outside it:

| Path | What it is | Who writes it |
|---|---|---|
| `hq/knowledge/.ledger.jsonl` | the knowledge base itself: every entry, every change, append-only | HQ, through `gtmux knowledge …` |
| `hq/knowledge/*.md` | the same entries rendered by topic, for reading | gtmux, overwritten on every render |
| `hq/knowledge/promotions/` | the take-away brief of an entry waiting to be carried somewhere | gtmux |
| `hq/knowledge/tools/` | scripts HQ wrote for itself, each named by one entry | HQ |
| `hq/AGENTS.md` | HQ's charter: how it works, shipped with gtmux | gtmux, replaced on update |
| `hq/LOCAL.md` | your own standing rules | you, by hand. gtmux never overwrites it |
| `hq/notes/board.md` | HQ's current picture of the fleet, not knowledge | HQ |
| `knowledge/machine.md` | the handful of lessons every agent on this machine must know | gtmux, generated |

Two of those folders are called `knowledge`, and the difference between them is not that
one is a copy of the other. It is that they have different readers, and the second reader
cannot see the first folder at all.

`hq/knowledge/` is HQ's own. No other agent looks in it; a Claude Code or Codex session
you open in some project does not know it exists. What every agent does read, at startup,
is its own global instruction file, `~/.claude/CLAUDE.md` for Claude Code. So the only
route from something HQ learned to the session actually doing the work runs through that
file, and `machine.md` is the staging post on that route.

The cost of not having it is easy to see. On a machine where `rm` is aliased to the
interactive version, an agent that runs it in a non-interactive shell fails silently: the
command looks like it ran and the file is still there. Recorded in the knowledge base
alone, that lesson stops nobody, because a new session never reads the knowledge base.
Carried out to `machine.md` and into each agent's instruction file, it is in front of
every new session from its first turn.

Most entries never travel this way. They are about how HQ itself works, and hundreds of
them in every agent's context would crowd out the work, so this list stays short: a few
entries out of hundreds. Delete `machine.md` and the next sync writes it again.

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
| HQ | HQ only | a section appended to your `LOCAL.md` |
| this machine | every agent on this Mac | `knowledge/machine.md`, plus a short block in each agent's global instruction file |
| a repository | agents working in that repo | a block in that repo's `AGENTS.md`, written but never committed |
| everyone | all gtmux users | a pre-filled GitHub issue for you to open |

Promotion writes a brief. Landing is gtmux actually carrying it, and the common case is
"this machine": your Claude Code, Codex, opencode and Kimi Code each get a block naming
the lessons and pointing at the full text, so a fresh session knows them without anyone
pasting anything. `gtmux knowledge carriers` shows each agent's file
and whether it is current; `gtmux doctor --fix` repairs a stale one.

An entry that turns out to be wrong is retired with a reason, and the reason survives:
the ledger is the only place a later reader can learn what was wrong with it.

## Reading it and changing it

On the phone and iPad, open HQ and tap the knowledge row. In the menu bar, the HQ card's
`KNOWLEDGE` row opens the same list. Both show what is waiting on you first.

From the terminal:

```sh
gtmux knowledge list                 # every live entry
gtmux knowledge list --topic pitfalls
gtmux knowledge show <id>            # one entry in full
gtmux knowledge lint                 # an audit of the base: what to fix, never fixed for you
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

Nothing in the base is uploaded anywhere. `gtmux hq --export` packs the whole home folder
into one file when you want to move machines, `--import` restores it, and `--records`
tells you how big it has grown.

## If you want to read further

[docs/cli.md](cli.md) has every command and flag. The design behind it, including why the
layers are separate and what happens at each gate, is in
[docs/design/knowledge-layers.md](design/knowledge-layers.md).
