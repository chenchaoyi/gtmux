# The writing rules for a knowledge entry become one table, read in three places

## Why

How a KB entry should read is written down twice, and the two copies do not agree.

One copy is a paragraph in HQ's charter (`kb-plain-language`), which HQ reads before it
writes. The other is the set of regular expressions in `internal/knowledge/voice.go`, which
`gtmux knowledge lint` runs afterwards. The charter says "do not string clauses on dashes";
the lint counts two dashes. The charter says bold is for the exception; the lint counts two
bold runs. A dozen other things the charter asks for are checked nowhere, and nothing makes
the two copies move together. Editing one leaves the other where it was.

Both copies also only say what not to do. The hand rewrite of 2026-09-16 went through 485
of 505 entries, and what made it land was the pair of sentences: this is what was written,
this is what it should have said. Neither copy carries a single example.

The third gap is what the base looks like once it is summarised. An entry reaches most
agents as one line in `machine.md`, rendered as `[kind] title — first line of the body`.
On this machine that line currently reads:

```
- [pitfalls] alias-makes-destructive-ops-silently-noop 交互式别名让破坏性操作静默不执行 —— 两种失效形态、四层受害者,只有改 shell 配置能一次拦住 — **本机 shell 把 `rm` / `cp` / `mv` 别名成带 `-i` 的交互式版本。**
```

One line carries the slug the id already holds, a dash the writer used to bolt an aside on,
a second dash the renderer added, and a whole sentence in bold. It is hard for a person to
read, and being hard to read did not buy the agent anything it can act on.

## What changes

**The rules become one table in the code.** `internal/knowledge/plainlang.go` holds every
rule as data: a number, what to do in one line, why, and a pair of sentences showing a
real before and after. English and Chinese halves, both written for their own reader.

**Each rule says how confidently it can be judged.** A rule is either mechanical (a matcher
can find it, so the lint runs it) or a judgement (only a reader can settle it, so the table
states it and no check pretends to). The tiering the lint already borrowed from the
humanizer skill stays: a strong tell counts on one sighting, a weak one needs company.

**Three readers, one table.** `gtmux knowledge lint` takes the mechanical rules from it.
`gtmux knowledge style` prints the whole table, with `--json` for an agent that wants it
structured. The charter paragraph shrinks to the judgement half and points at the command,
so there is nothing left to drift.

**The rules name the two readers an entry has.** A KB entry is read by a person who wants
to understand and by an agent that wants to comply, and plain language must not cost the
second one anything. Two rules that the humanizer skill has no reason to carry go in:
everything executable stays verbatim (commands, paths, thresholds, ids, error text, the
commander's own words), and an entry states who, where, what happened and what to do
instead, because an entry nobody can act on is not worth the base it sits in.

**Three more things a matcher can honestly find** join the lint: a guess presented as a
fact, a title that the body's first sentence restates, and an implementation name used as a
title. Measured against the real base of 720 entries they report 12, 12 and 24 entries.

**The fourth one turned out to be the renderer's fault, not the writer's.** A check for the
id's slug in the title fired on 457 of those 720 entries, and that is not drift: the machine
index never printed the id, so writing the slug into the title was the only way an agent
reading that line could look the entry up. The index prints the id now and the render drops
the copy from the title, so 457 entries read better with nobody editing them. The rule stays
in the table as a judgement, for the next entry rather than as a backlog nobody would work
down.

**The summary line carries what the line is for.** The title goes on its own line, and under
it the summary and the id, in the order they are needed. It used to be title, dash, summary,
with no id anywhere.

## Not in this change

- **Rewriting any entry.** The lint reports and the verbs write; that line does not move.
  The findings this adds are a backlog with a number, worked down by hand.
- **Judging whether a sentence says anything.** No check claims to. The table states the
  judgement rules so a reader has them, and leaves the reading to the reader.
- **A rule about the commander's own voice.** `LOCAL.md` is the authority on how he wants
  to be written to, and stays above this table, as it already does.

## Surfaces

- **终端 / terminal** — where the change lives: `gtmux knowledge style` is new, `gtmux knowledge
  lint` gains four checks. Over a remote `attach` it is the same terminal, so nothing
  further is needed.
- **菜单栏 / menubar** — no UI change. The HQ card already shows the lint counts it is given, and
  the new checks appear there as more of what it already renders.
- **手机 / phone** — not applicable. The phone reads the base and cannot write to it, so a writing
  rule has no surface there.
- **iPad** — not applicable, for the same reason as the phone.
- **Web** — not applicable. The shared web page is scoped to panes and never shows the base.
