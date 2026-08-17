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
- **The user's language needs no gtmux-specific setup, and the machine speaks
  ONE language**: the resolved language is `--lang`, else `GTMUX_LANG`, else
  `gtmux config lang` (the machine-level choice in config.json — the layer a
  launchd-started serve and a hook subprocess share with the user's shell,
  since none of them share an environment), else the system locale (`LC_ALL`
  over `LANG`, POSIX order — a `zh*` prefix reads Chinese), else English. A
  set-but-unknown value at an explicit layer resolves to English rather than
  silently falling through — except the config value `auto`, which
  deliberately means "follow the locale".
- **The charter ships whole in the user's language**: one
  complete English edition and one complete Chinese edition, sharing one
  markdown skeleton and one actionable anchor set (every command, wake class,
  control record, and signal glyph identical — only prose translates). The
  managed AGENTS.md marker records the installed language, and `gtmux hq`
  regenerates the file when the version OR the language differs (backup first);
  a pre-bilingual marker without a language regenerates once. A same-version
  language switch SHALL be announced as a language switch, not as a
  same-number upgrade. LOCAL.md and the user's notes are never touched by
  regeneration.
- **LOCAL.md seeds in the user's language too**: the personalization template
  (the explainer and its commented examples) follows the resolved language at
  seed time, and seed-once is absolute — an existing LOCAL.md is never
  rewritten by a later language switch; the file is the user's from the first
  byte.
- **The board seeds in the user's language**, and the language
  discipline taught is "keep the board in the language it was seeded in — never
  flip an existing board", preserving the don't-machine-translate lesson
  without mandating any particular language.
- **Enforced, not reviewed back in:** a test SHALL ban the known author-leak
  tokens from the seeds and BOTH charter editions, and a structural guard SHALL
  fail the build when the two editions' skeletons or anchor sets drift apart.

#### Scenario: A fresh user's first day is their own blank page

- **WHEN** `gtmux hq` seeds a brand-new home
- **THEN** every topic file is a template describing its purpose, no seed
  carries another person's lessons or vendor list, and the board's headings,
  the charter, and the LOCAL.md template are in the user's language

#### Scenario: A leaked author token fails the build

- **WHEN** a banned author-specific token is reintroduced into a seed or either
  charter edition
- **THEN** the ban-list test fails

#### Scenario: A language switch regenerates the charter

- **WHEN** `gtmux hq` runs with the resolved language differing from what the
  installed playbook's marker records — even at the same version
- **THEN** the managed AGENTS.md is regenerated in the new language with the
  prior file backed up, the notice names the language switch rather than a
  same-number upgrade, and LOCAL.md is untouched

#### Scenario: Charter drift is a red build

- **WHEN** a command, wake class, control record, signal glyph, or heading is
  added to one charter edition and not the other
- **THEN** the structural guard test fails

#### Scenario: A zh locale reads Chinese without setup

- **WHEN** `GTMUX_LANG` is unset and the system locale is `zh_CN.UTF-8`
- **THEN** gtmux's output, the charter, the board seed, and the LOCAL.md
  template are Chinese — no gtmux-specific environment variable required

#### Scenario: Every process resolves the same language

- **WHEN** `gtmux config lang zh` is set and a launchd-started serve (no
  GTMUX_LANG, no user locale) emits a wake suffix or a desktop notification
- **THEN** it is Chinese — the same language the user's own shell resolves
