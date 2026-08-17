# language-loose-ends — the last two surfaces that ignored the user's language

Origin: the commander's second whole-of-hq review (2026-08-17), findings #3
and #4. bilingual-playbook made the charter follow `GTMUX_LANG`; two small
surfaces did not follow it there.

## Why

1. **LOCAL.md's template is English-only.** The charter and the board both
   seed in the user's language; the one file that is THE USER'S OWN — the
   personalization slot the first-launch interview writes into — seeded with
   an English explainer and English examples. A Chinese user's first launch:
   a Chinese charter asks three questions in Chinese, and writes the answers
   into a file that opens in English.
2. **A same-version language switch announces itself as "Upgraded … v32 →
   v32"** — the version did not change and the reason (the language) goes
   unsaid.

## What changes

- `hqLocalTemplate` becomes language-aware (en/zh editions, the board-seed
  pattern). Seed-once is preserved: an existing LOCAL.md is never rewritten by
  a later language switch — the file is the user's from the first byte.
- The upgrade records a language switch (`seedResult.LangSwitch`), and a
  same-version switch prints "Switched the HQ charter language to zh …"
  instead of a same-number upgrade line.
- Spec: the supervisor-agent language requirement adds the LOCAL.md-template
  clause.

## Non-goals

No retroactive translation of an existing LOCAL.md (it is the user's file);
no new i18n for the knowledge seeds (hq-first-person's scope decision stands).

## Impact / risk

Text plus one notice branch; both behaviors are pinned by tests (template
follows SetLang, seed-once survives a switch, the notice names the switch).
