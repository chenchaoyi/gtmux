# Change: kb-bilingual

> STATUS: proposed 2026-09-14. Design record: `design.md` here (D1–D7).

## Why

The knowledge base is written by HQ in the commander's working language — on the machine
this was designed on, every one of 470 entries is Chinese — and read on five surfaces
whose CHROME already follows the reader's language (CLI `GTMUX_LANG`, the menu bar's
language setting, the phone's device language) while the CONTENT does not. Switch the app
to English and the labels change; the entries do not. Two consequences:

1. **A reader who set English gets a half-translated screen.** The design docs and user
   docs became bilingual pairs on 2026-09-13 (`docs/design/*`, `docs/*`); the one body of
   text that stayed single-language is the product's own memory.
2. **What leaves the machine leaves in the wrong language.** An `everyone` promotion
   becomes a GitHub issue on a public, English repository; the brief and the issue prefill
   carry the entry as written. A `repo` block written into a shared repository's
   `AGENTS.md` is read by whoever works there.

The commander's ask (2026-09-14): 「kb 也需要支持双语言切换」 — the knowledge base too
must switch language, not only its chrome.

## What Changes

1. **An entry carries two languages.** Each ledger entry records the language it was
   written in (`lang`) and may carry the other language as `alt` (title + body). Additive
   fields; the ledger format stays v2; old entries get their `lang` inferred at read time
   (CJK ratio) and never rewritten.
2. **Reading picks the reader's language.** CLI text output follows `GTMUX_LANG` (with
   `--lang` to override); `--json` and `GET /api/hq/knowledge*` carry both halves and the
   source tag, and each client picks by its own language setting. A missing half falls
   back to the source, marked so the reader knows.
3. **HQ writes both.** `add` and `supersede` take `--alt-lang / --alt-title / --alt-body(-file)`;
   a new `alt <id>` verb adds or replaces the other half of an existing entry, so the
   existing base can be backfilled in batches by HQ's own cadence. The playbook (v41)
   teaches: write the entry in your working language, then the other; it is the same
   discipline as the design docs — written, not translated.
4. **Distribution and exits choose deliberately** (D4): the machine's canonical file and
   the agents' instruction blocks render in the machine's language; the `everyone` brief
   and the issue prefill render in English when it exists; a `repo` block renders in the
   machine's language (that repository's readers are the same people).
5. **Lint says what is missing:** a `monolingual` finding per entry with no other half,
   counted in the self-check summary; search (`neighbours`, the phone's find, the menu
   bar's find) matches both halves.
6. **Menu bar and phone:** the knowledge sheets show the half matching the app's
   language, fall back with a small tag (`zh` / `en`) when only the other exists, and the
   app's existing language setting is the switch — no second control.

## Surfaces

- 终端 (terminal / attach)：`gtmux knowledge` 的文本输出随 `GTMUX_LANG`，`--lang` 可覆盖；`--json` 两种都带。远程 attach 不涉及。
- 菜单栏 (menubar)：知识库窗口按 app 语言设置显示对应半份，缺的一半回落并打标；搜索两种都匹配。
- 手机 (phone)：知识库弹层同上，随设备语言 / app 语言三态。
- iPad：同一份 `KnowledgeSheet`，随手机一起。
- Web：不适用 —— 共享页不显示知识库。

## What does NOT change

- The ledger's append-only shape and version; every op stays readable by v1.0.18.
- The three axes (kind / provenance / audience), promotion and landing.
- gtmux never calls a model: the second half is written by HQ, never machine-translated.

## Impact

- `internal/knowledge` (op fields, fold, render per language, api rows, lint, verbs),
  `internal/server/hq.go` (unchanged shape plus the new fields), `internal/hq/hq.go`
  playbook v41, `macapp` KB window, `mobileapp` KnowledgeSheet + client types,
  `docs/cli.md` pair, `docs/design/knowledge-layers.md` pair, specs `hq-knowledge`,
  `menu-bar-app`, `mobile-app`.
