# Change: kb-sensitive-entries

> STATUS: implemented 2026-09-14 (same PR). On the commander's direction 「KB 可以记录敏感信息，但是要求用户确认好」, after 「我还是希望 hq 的 kb 能够记住一些用户自己的敏感信息，为了方便，而且电脑是自己的」.

## Why

The charter said NEVER store secrets. The commander wants HQ's knowledge base to hold his own
detail — accounts, personal facts, credentials he chooses to keep there — because the machine
is his and looking them up elsewhere is friction. What he asked for in return is that nothing
of the kind goes in without his say-so. Two facts shape the design: the base is deliberately
distributed (machine.md, every agent's instruction block, `everyone` issues), so a sensitive
entry needs a hard wall between "in the ledger" and "leaves the machine"; and the ledger
cannot know that a person said yes, but it can refuse a sensitive write that carries no record
of asking.

## What Changes

1. **A `sensitive` mark on entries**, set at `add`/`supersede` with `--sensitive`, or later
   with `sensitive <id>` (and removed with `--off`). Every such write requires
   `--confirmed "<the commander's own words>"`, stored on the entry: the record that HQ asked.
   A superseded sensitive entry passes the mark and the words to its successor.
2. **A sensitive entry never leaves the machine.** `promote` refuses any audience but `hq`;
   `machine.md` and the repo blocks skip it regardless; the API row carries `sensitive` so
   the Mac and the phone show a lock and say "this Mac only".
3. **Lint `unmarked-sensitive`**: an entry that reads like a credential (a bearer token, a
   `password=`/`token=` value, a PEM key, common vendor key prefixes) without the mark — one
   written without asking. Narrow on purpose: the word "password" is not a credential.
4. **Charter v43** replaces "NEVER store secrets" with: the commander's own detail goes in
   only after they confirm, shown the exact title and body, never from a worker's capture or
   a wake; it stays here; other people's secrets still stay out (pointer only).

## Surfaces

- 终端 (terminal, incl. remote attach): `--sensitive --confirmed`, the `sensitive` verb, the
  promote refusal, the lint check, the charter.
- 菜单栏 (menubar): the knowledge window shows a lock on the row and "sensitive · this Mac
  only" in the entry's axes line; promoting one past `hq` shows the CLI's refusal verbatim.
- 手机 (phone): the knowledge sheet shows "sensitive ·" on the row and in the axes line.
- iPad: same sheet, same marks.
- Web: not applicable — the shared page carries no knowledge base.

## What does NOT change

- Nothing is encrypted on disk; no entry is redacted from the commander's own surfaces.
- The export lock (hq-export-passphrase) is unchanged and remains the protection for the copy
  that travels.
