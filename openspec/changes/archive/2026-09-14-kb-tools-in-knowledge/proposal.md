# Change: kb-tools-in-knowledge

> STATUS: implemented 2026-09-14 (same PR). On the commander's direction 「或者只有 tools 归到 knowledge 呢？没有 knowledge 的 tools 是不是也没啥意义」 · 「design 这个确实感觉有点多余 … 归到 notes 就好」 · 「先把这个弄了吧」.

## Why

HQ's home on the design machine had grown two folders the charter never defined: `tools/`
(fourteen scripts HQ wrote, with a `README.md` as their index, and a LOCAL.md iron rule
"read `tools/README.md` before dispatching") and `designs/` (one brief). Meanwhile the
knowledge base already pointed at `tools/` thirty times. So "what can I do" was indexed in
two places, and a reader had to check both — the shape of every "there was a tool for that
and nobody used it" incident. A script is executable know-how; without the entry that says
when to run it, it is a file nobody will find. And a brief is a note.

## What Changes

1. **The charter (v42) defines the home's top level**: `AGENTS.md`, `LOCAL.md`, `notes/`,
   `knowledge/` — nothing else. Scripts live in `knowledge/tools/`, one `howto` entry per
   script naming its path and when to run it; the ledger is the only index. An inherited
   top-level `tools/` or `designs/` is moved once, by HQ, with an entry per script and the
   README's index retired into those entries.
2. **`gtmux knowledge lint` checks the pairing**: `orphan-tool` (a script under
   `knowledge/tools/` no live entry names) and `broken-tool` (an entry naming a
   `tools/<script>` that is not on disk). A base with no tools folder raises neither.
3. Nothing moves files automatically: the LOCAL.md rule that names `tools/README.md` is
   the commander's to edit, and the entries are prose HQ writes.

## Surfaces

- 终端 (terminal, incl. remote attach): `gtmux knowledge lint` gains the two checks; the
  charter text changes (regenerated on the next `gtmux hq`).
- 菜单栏 (menubar): unchanged — the knowledge window shows the new howto entries as any.
- 手机 (phone): unchanged, same reason.
- iPad: unchanged.
- Web: not applicable.

## What does NOT change

- The ledger format, the verbs, distribution. `notes/` and the board.
- No file is moved by gtmux; HQ does the one-time move under the charter's instruction.
