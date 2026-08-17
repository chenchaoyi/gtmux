# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: The seeds and playbook speak to any user

The seeded knowledge scaffold and the managed playbook SHALL be written for an
ARBITRARY user, not for gtmux's author or this machine's history:

- **Seeds are domain-neutral templates.** A topic seed describes WHAT belongs
  there, never WHOSE accounts, stacks, or workflows; no seed ships pre-filled
  lessons, undefined internal markers, or a particular vendor's names as the
  canonical examples.
- **Evidence never impersonates the reader.** The playbook may cite a measured
  incident as mechanism-plus-history ("a real dispatch died exactly this way,
  during development, …"), and SHALL NOT phrase it as the reader's own past
  ("a dispatch of yours", "already in your own knowledge base") — on a fresh
  machine such claims are false and an HQ that believes them will recount
  history that never happened there.
- **The charter ships whole in the user's language** (`GTMUX_LANG`): one
  complete English edition and one complete Chinese edition, sharing one
  markdown skeleton and one actionable anchor set (every command, wake class,
  control record, and signal glyph identical — only prose translates). The
  managed AGENTS.md marker records the installed language, and `gtmux hq`
  regenerates the file when the version OR the language differs (backup first);
  a pre-bilingual marker without a language regenerates once. LOCAL.md and the
  user's notes are never touched by regeneration.
- **The board seeds in the user's language** (`GTMUX_LANG`), and the language
  discipline taught is "keep the board in the language it was seeded in — never
  flip an existing board", preserving the don't-machine-translate lesson
  without mandating any particular language.
- **Enforced, not reviewed back in:** a test SHALL ban the known author-leak
  tokens from the seeds and BOTH charter editions, and a structural guard SHALL
  fail the build when the two editions' skeletons or anchor sets drift apart.

#### Scenario: A fresh user's first day is their own blank page

- **WHEN** `gtmux hq` seeds a brand-new home
- **THEN** every topic file is a template describing its purpose, no seed
  carries another person's lessons or vendor list, and the board's headings and
  the charter are in the user's language

#### Scenario: A leaked author token fails the build

- **WHEN** a banned author-specific token is reintroduced into a seed or either
  charter edition
- **THEN** the ban-list test fails

#### Scenario: A language switch regenerates the charter

- **WHEN** `gtmux hq` runs with `GTMUX_LANG` naming a different language than
  the installed playbook's marker records — even at the same version
- **THEN** the managed AGENTS.md is regenerated in the new language with the
  prior file backed up, and LOCAL.md is untouched

#### Scenario: Charter drift is a red build

- **WHEN** a command, wake class, control record, signal glyph, or heading is
  added to one charter edition and not the other
- **THEN** the structural guard test fails
