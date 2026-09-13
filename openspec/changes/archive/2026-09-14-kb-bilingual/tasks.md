# Tasks — kb-bilingual

## Phase 1 — ledger and verbs
- [x] 1.1 `lang` + `alt` on `knowledgeOp`; fold rules; read-time `langAssumed` inference (CJK ratio); golden test unchanged for old ledgers
- [x] 1.2 `add` / `supersede` `--alt-lang --alt-title --alt-body(-file)`; new `alt <id>` verb; validation (alt.lang ≠ lang; both title and body)
- [x] 1.3 Resolver `pick(op, lang)` with the three-step rule and the fallback tag; `--lang` flag on list/show/lint

## Phase 2 — outputs
- [x] 2.1 Topic files, `machine.md`, carriers' blocks, `LOCAL.md` landing render in the machine's language (D4); block hash covers it
- [x] 2.2 `everyone` brief and issue prefill in English when present
- [x] 2.3 JSON rows and `/api/hq/knowledge*` carry `lang`, `alt`, `langAssumed`; `api/contract.md`
- [x] 2.4 Lint `monolingual`; self-check summary; `neighbours` and find match both halves

## Phase 3 — surfaces
- [x] 3.1 Menu bar KB window: resolve by `L10n.lang`, fallback tag, find over both halves
- [x] 3.2 Phone/iPad KnowledgeSheet: resolve by `lang`, fallback tag, find over both halves; demo data gets an `alt` on a few entries
- [x] 3.3 Playbook v41: bilingual entries; backfill cadence via lint

## Phase 4 — docs, specs, verification
- [x] 4.1 `docs/cli.md` pair, `docs/design/knowledge-layers.md` pair, MOBILE/DESIGN KB sections
- [x] 4.2 Specs: hq-knowledge, menu-bar-app, mobile-app synced; archive
- [x] 4.3 CLI: covered by the package tests (add with alt, alt verb, show/list resolution, lint monolingual, machine-language render); phone/Mac: model tests in both languages; the demo carries one bilingual entry (k5) so a simulator run in either language shows the switch — not run this round
