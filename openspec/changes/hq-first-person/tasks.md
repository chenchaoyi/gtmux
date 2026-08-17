# Tasks — hq-first-person

Status: IMPLEMENTED (2026-08-17). The keeper is the ban-list test: neutrality is
enforced, not reviewed back in.

## 1. Neutral seeds + honest memory

- [x] 1.1 Seeds neutralized: accounts/workflows/README topic descriptors lose the
      author's stack; `best-practices.md` ships as a clean template (the two
      portable budget rules fold into Policy #3); `environment.md` shrinks to
      purpose + one `agent-proxy` pointer; `corrections.md` teaches the current
      verbs (promote included).
- [x] 1.2 False-memory passages → mechanism + dated development incident
      ("of yours"/"your own knowledge base" gone); example slugs neutralized
      (`review-pr-518`, `menubar-width`); the 2026-08-03 incident marked as
      development history; agent-command names get one "your agent's
      equivalents" parenthetical.
- [x] 1.3 Ban-list regression test over `hqInstructions` + every seed: the
      author-leak tokens (Apple, Cloudflare, Appium, `(B2)`, the old slugs,
      "of yours", "your own knowledge base") fail the build if they return.

## 2. The board speaks the user's language

- [x] 2.1 The board seed renders its headings/columns via i18n (`GTMUX_LANG`)
      at seed time; the playbook's language rule becomes "keep the board in the
      language it was seeded in — never flip an existing board", keeping the
      don't-machine-translate lesson. Tests: zh and en seeds.

## 3. LOCAL.md discoverable, asked-for

- [x] 3.1 Template: three commented examples (reporting style · quiet hours ·
      domains/repos) + the boundary note (governs the supervisor only;
      notifications are `config.json`'s).
- [x] 3.2 First-run notice names `knowledge/`, `LOCAL.md`, and one
      `gtmux capture` example (en+zh).
- [x] 3.3 Startup briefing: when LOCAL.md is still the seeded template, HQ asks
      the three personalization questions and writes the answers into LOCAL.md;
      an edited LOCAL.md skips the step. Playbook v27 → v28.
- [x] 3.4 "local notes" → the literal `notes/` throughout the playbook and
      seeds — the LOCAL.md name collision ends.

## 4. Docs (same PR)

- [x] 4.1 README(+zh) hq section: "edit AGENTS.md" → LOCAL.md; one knowledge-
      base sentence; `gtmux serve` named as what drives the periodic rituals.
      `docs/cli.md`'s same AGENTS.md line fixed.
- [x] 4.2 Doc debts: the dead `openspec/changes/hq-capture-loop` link; a `⟣`
      signal-glyph legend in the hq section; `hqWake` keys join the config
      section.
- [x] 4.3 Spec deltas: supervisor-agent ADDED (seeds/playbook neutrality) +
      MODIFIED (startup briefing gains the first-launch questions).
- [x] 4.4 Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
      `scripts/check-design.sh`, `openspec validate --strict`.
