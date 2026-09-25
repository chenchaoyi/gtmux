# One retrieval for the knowledge base, and it works in Chinese

## Why

The base has 720 entries and two separate ways of searching them, written independently and
sharing nothing.

`neighbours.go` tokenizes both sides (ASCII words, CJK bigrams) and ranks by overlap. It is
what HQ gets from `knowledge neighbours`, and what `add` shows before it writes.

`knowledgematch.go` is the echo printed when HQ spawns work. It reads the rendered topic
files line by line and keeps a bullet if it contains the repo name or a goal keyword, where
a goal keyword is whatever `strings.Fields` produced. A Chinese goal has no spaces, so the
whole sentence becomes one keyword and the substring match never fires:

```
"fix the pairing timeout on the phone"  -> [pairing timeout phone]   matches
"修复手机端配对超时"                      -> [修复手机端配对超时]        matches nothing, ever
```

Recall for a Chinese goal is zero. The echo also reads only `pitfalls`, `workflows` and the
declared custom topics, so the 257 entries under `best-practices` are never echoed at all.
Between them, 377 of 720 entries cannot reach a dispatch.

The path that does work is not doing well either. Measured against the 904 `[[link]]` pairs
HQ drew by hand, `neighbours` returns a known-related entry in its top 5 for 36% of them and
does not return it at all for 63%. The reason is not scale: at 467 entries the linked pairs
sat at p50 0.17 against random pairs at p90 0.13, and at 720 they sit at 0.191 against
0.143. The two moved together. The ranking signal simply has this much precision, because
every token counts the same: `chisel` and `pane` weigh alike, and at the shipped floor of
0.14 about 81 of 719 entries clear it on any query.

## What changes

**One retrieval, in one file.** `internal/knowledge/retrieve.go` owns tokenizing, the corpus
statistics and the scoring. `neighbours` and the dispatch echo both call it, and neither
keeps a private notion of what a match is.

**Rare words count for more.** Scoring weights each shared token by its inverse document
frequency over the live base, so a token two entries share tells us more when few other
entries have it. Measured offline against the same 904 pairs: recall@5 goes from 33% to 43%,
recall@10 from 43% to 53%, recall@15 from 50% to 61%.

**The echo searches the base instead of grepping its renders.** It ranks live entries with
the shared retrieval, which means a Chinese goal works for the first time, and it covers
`best-practices` as well. `accounts`, `corrections` and `environment` stay out: those were
excluded on purpose, and the reason has not changed.

**`knowledge search "<text>"`** is the verb for asking the base a question in words.
`neighbours <id>` keeps doing what it does, which is a different question: what is near this
entry.

**`neighbours` returns ten.** Five was set when nothing measured how often the right entry
sat sixth.

## Not in this change

- **A vector store.** The CLI must stay cgo-free, which rules out a local embedding model,
  and an embedding API would send entries off this machine, which the sensitive-entry rule
  forbids. Its gain also has a cheaper approximation that had not been tried until now. The
  2026-09-12 research said to revisit with measured evidence; this is the evidence, and it
  says do the cheap thing first and measure again.
- **Putting KB hits into the dispatched agent's payload.** The echo prints where HQ can see
  it, and HQ decides what to relay. Changing that changes what a worker receives, which is a
  separate judgement.
- **Retiring the overlap floor.** It stays, on the weighted score, at the value the
  measurement supports.

## Surfaces

- **终端 / terminal** — where this lives: `knowledge search` is new, `neighbours` returns
  ten and ranks better, and the spawn echo finally answers a Chinese goal.
- **菜单栏 / menubar** — no change. The HQ card reads counts and lint summaries, neither of
  which this touches.
- **手机 / phone** — no change. The phone browses the base by topic and reads entries; it
  does not rank them.
- **iPad** — no change, same as the phone.
- **Web** — not applicable. The shared web page is scoped to panes and never shows the base.
