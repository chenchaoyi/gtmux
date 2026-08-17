# bilingual-playbook — the charter speaks the user's language, whole

Origin: the commander's whole-of-hq review, deferred from hq-first-person
(whose non-goal said "a single mixed-language charter remains"). This clears
that last language debt: the supervisor's playbook itself.

## Why

1. **The charter is mixed-language for everyone.** `hqInstructions` is ~600
   lines of English prose with Chinese asides appended to many bullets — right
   for the commander who reads both, noise for a user who reads one. An
   English-only reader wades past 中文 they cannot read; a Chinese-only reader
   gets a charter that is 80% English.
2. **The board already follows the user** (`GTMUX_LANG`, hq-first-person); the
   charter — the document that TEACHES the board discipline — does not.
3. **A supervisor reasons best in the language it is instructed in.** The
   playbook is HQ's behavior specification; delivering it whole in one
   language removes the constant register-switching the mixed charter forces.

## What changes

- **Two complete charters, one structure.** `hqInstructionsEN` (the current
  text, Chinese asides removed) and `hqInstructionsZH` (a full translation).
  `hqInstructions` stays as the EN alias so the existing anchor tests keep
  their meaning. Command names, event names, wake classes, signal glyphs
  (`»`, `⟣`), and the markdown skeleton are IDENTICAL across the two — only
  prose translates.
- **The playbook marker carries the language**:
  `<!-- gtmux-hq-playbook v31 lang:zh · … -->`. `gtmux hq` regenerates the
  managed AGENTS.md when the version OR the language differs from the
  installed one (backup first, keyed by version+lang); a legacy marker without
  `lang:` reads as needing regeneration once. LOCAL.md and the user's notes
  are untouched, as ever.
- **Drift is mechanically impossible to miss**: a guard test asserts the two
  charters agree on the markdown heading sequence and on the exact set of
  backtick anchors (commands, wake classes, control records) — a bullet added
  to one charter and not the other is a red build. The ban-list tests
  (author-leak, retired vocabulary) run over BOTH charters.
- `hqPlaybookVersion` 30 → 31.

## Non-goals

The knowledge seeds stay as they are (domain-neutral English templates —
hq-first-person's scope decision stands); LOCAL.md's template already carries
en+zh where it matters. No per-section language mixing: GTMUX_LANG picks ONE
charter whole. Runtime CLI output (i18n.Say/Tr) is already bilingual and is
not touched.

## Impact / risk

Text-heavy; the two mechanism changes are the lang-aware marker/upgrade and
the EN/ZH selection in `generatedPlaybook()`. The risk of a translation
changing HQ behavior is bounded by the anchor-set guard (every command and
class HQ can act on is byte-identical across charters) and by the shared
skeleton. Existing playbook tests anchor the EN charter; ZH gains its own
structure/anchor coverage.
