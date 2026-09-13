# Design — kb-bilingual

Each decision names what it rejected and why.

## D1 — Source language plus one alternate, additive on the same ledger version

`knowledgeOp` gains `lang` ("zh" | "en", the language the entry was written in) and `alt`
(`{lang, title, body}`), both `omitempty`. Fold: `add` sets them, `supersede` inherits then
overrides, a new `alt` op replaces the alternate half. Entries without `lang` (everything
written before this change) get it inferred at read time from the CJK ratio of title+body
and are marked `langAssumed`, the same read-time migration the kinds got in
`hq-knowledge-engine`; the ledger is never rewritten.

Rejected: a map of arbitrary languages. The product ships in two; a third would be a
product decision, not a schema one, and a map would make every reader handle "which one
do I show" with no primary. Rejected: two ledgers, one per language — a supersede chain
split across files is the drift this change exists to prevent.

## D2 — HQ writes both halves; gtmux never translates

gtmux calls no model (a standing principle). The alternate half is prose HQ writes at
`add` time (`--alt-lang en --alt-title … --alt-body-file -`) or later with `gtmux knowledge
alt <id> --lang en --title … --body-file -`. The playbook (v41) states the discipline: write
the entry in your working language first, the other second; it is written for its reader,
not translated — the same rule the design docs follow. Cost: one more paragraph per entry
from HQ. The base's 470 existing entries are backfilled by HQ in batches on its own cadence,
guided by lint's `monolingual` count; nothing blocks on it.

Rejected: mandatory bilingual `add` (a refused add loses the lesson at the moment it is
learned; the correction→charter loop cannot afford that).

## D3 — Readers resolve; the API carries both

Text output (`knowledge list/show/lint`, topic files) resolves in the CLI by `GTMUX_LANG`
with `--lang zh|en` to override: source half if it matches, else the alternate if it
matches, else the source with a `[zh]`/`[en]` tag. JSON (`--json`, `/api/hq/knowledge*`)
carries `lang`, `title`, `body` (source) and `alt` as written, plus `langAssumed`; the menu
bar and the phone resolve with the same three-step rule against their own language
setting. Rejected: resolving server-side from a `?lang=` parameter — the menu bar reaches
the CLI with no language of its own today, and the phone's setting changes at runtime; a
client that holds both halves switches without a refetch.

## D4 — Where each output goes, which language it takes

| Output | Language |
|---|---|
| `machine.md` and the agents' instruction blocks | the machine's: the majority language of the base's live entries (not the syncing process's `GTMUX_LANG` — the daily sync runs under launchd with none set, and a block that flipped per renderer would churn every agent's file); fallback to source |
| `repo` block written into a repository's `AGENTS.md` | the machine's; that repository's readers are the same people |
| `hq` landing into `LOCAL.md` | the machine's |
| `everyone` brief and the issue prefill | English when it exists, else source — the target is a public English repository |
| topic files under `knowledge/` | the machine's |

A sync re-renders when the base's majority language changes, the same way it re-renders
on content change (the block hash covers the rendered text).

## D5 — Lint

`monolingual`: an entry with no alternate half. Reported per entry, counted in the
self-check one-liner ("… 470 monolingual"), never auto-fixed. `neighbours` and the two
apps' find match against both halves, so a duplicate written in the other language is
still found.

## D6 — Chrome stays where it is

No new language control on any surface: the CLI's `GTMUX_LANG`, the menu bar's language
setting and the phone's device/app language already exist and already flip the chrome.
The content follows the same switch. The tag on a fallback entry (`zh`/`en`, small, beside
the axes line) is the only new visible element.

## D7 — Five surfaces

Terminal: CLI output resolves by language. Menu bar and phone/iPad: the same sheet
component per surface resolves client-side. Web: no knowledge base on the share page;
not applicable.
