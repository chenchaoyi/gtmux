# Tasks — hq-open-topics

Status: IMPLEMENTED (2026-08-17, one pass). The risky direction is the topic-validation seam drifting
between capture and knowledge again; one function over one source, pinned by a
whole-loop test.

## 1. The topic operation

- [x] 1.1 Ledger op `topic` (`id` = the topic name, `title` = its one-line
      description); name validation: slug charset `[a-z0-9-]`, ≤ 40 bytes,
      refuses built-ins, existing customs, and the reserved directory names
      (`README`, `legacy`, `promotions`), loudly.
- [x] 1.2 One read, one source: a shared state loader returns (live entries,
      custom topics); `validKnowledgeTopic` takes the custom set; capture and
      every knowledge verb validate through it. Capture accepts `environment`
      and custom topics (the 5-vs-6 inconsistency closes).
- [x] 1.3 Tests: declare/fold; every name refusal; the shared validation from
      BOTH entrances (a custom topic accepted by capture and by add; a bogus one
      refused by both with the vocabulary named).

## 2. Renders + echo

- [x] 2.1 A declared topic renders immediately: marker + `# <name>` + the
      description as an intro line; custom topics always render (they are
      explicit intent, unlike untouched built-in seeds).
- [x] 2.2 The dispatch echo consults every custom topic alongside
      pitfalls/workflows; the built-in exclusions stand. Tests: a custom-topic
      lesson echoes by repo/keyword; accounts/corrections still do not; the
      line cap holds across the widened set.
- [x] 2.3 Whole-loop test: `topic datasets` → worker-cwd `gtmux capture "… @datasets"`
      → `add --capture` → the entry renders under `datasets.md` with provenance
      → `MatchKnowledge` surfaces it.

## 3. The verb + teaching + docs (same PR)

- [x] 3.1 `gtmux knowledge topic <name> --desc "…"` — mutation through the shared
      HQ-home gate + commit path (ledger, renders, `gtmux:audit:knowledge`);
      usage text (en+zh) gains it; `list --topic` and error messages name the
      full current vocabulary.
- [x] 3.2 Seeded README: "Add topic files as needed" → the verb; playbook KB
      section teaches it; `hqPlaybookVersion` 25 → 26.
- [x] 3.3 `docs/cli.md`: the knowledge section documents the vocabulary rule and
      the verb; the capture section drops the fixed five-topic list for the
      shared rule.
- [x] 3.4 Spec deltas: hq-knowledge (ledger + verbs requirements gain the topic
      op; an ADDED requirement for the extensible vocabulary); supervisor-agent
      capture-spool requirement (topic ∈ built-ins ∪ declared).
- [x] 3.5 Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
      `scripts/check-design.sh`, `openspec validate --strict`.
