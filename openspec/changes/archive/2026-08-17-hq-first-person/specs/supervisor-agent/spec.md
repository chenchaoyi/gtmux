# supervisor-agent (delta)

## ADDED Requirements

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
- **The board seeds in the user's language** (`GTMUX_LANG`), and the language
  discipline taught is "keep the board in the language it was seeded in — never
  flip an existing board", preserving the don't-machine-translate lesson
  without mandating any particular language.
- **Enforced, not reviewed back in:** a test SHALL ban the known author-leak
  tokens from the seeds and the playbook, so a neutrality regression is a red
  build.

#### Scenario: A fresh user's first day is their own blank page

- **WHEN** `gtmux hq` seeds a brand-new home
- **THEN** every topic file is a template describing its purpose, no seed
  carries another person's lessons or vendor list, and the board's headings are
  in the user's language

#### Scenario: A leaked author token fails the build

- **WHEN** a banned author-specific token is reintroduced into a seed or the
  playbook
- **THEN** the ban-list test fails

## MODIFIED Requirements

### Requirement: HQ opens with a self-introduction and status briefing

The system SHALL, on the supervisor's FIRST turn (a fresh spawn, OR a relaunch of a
stamped-but-dead HQ pane), deliver a MINIMAL, agent-agnostic TRIGGER — a single
`» gtmux·startup` signal line, NOT a large multi-line prompt — after the agent comes up
(via the verified dispatch path: wait-for-ready, then a land-verified deliver, the same
`gtmux spawn` uses). The briefing's CONTENT and format SHALL live in
the seeded playbook (AGENTS.md's "## First turn" section, which every agent reads
through its own convention file — Claude via CLAUDE.md→AGENTS.md, Codex/Cursor/Amp
natively), defining the two-part first output: (a) a one/two-sentence self-introduction
of the supervisor role (sense · decide · dispatch · supervise · report + curate the
knowledge base), and (b) an immediate status report EXACTLY per the playbook's status
policy — a COLUMN-ALIGNED TABLE from `gtmux digest --json` / `gtmux usage --json` /
`gtmux limits --json` (needs-you first, token-usage rollup + subscription-window room),
never a prose paragraph. Keeping the content in the playbook and the injected text a
one-line trigger makes it ROBUST (a one-liner submits reliably where a big multi-line
paste was flaky — typed-but-not-submitted) and AGENT-AGNOSTIC (not Claude-Code-specific).

The playbook SHALL additionally teach a FIRST-LAUNCH personalization step: when
`LOCAL.md` is still the seeded template (no user content — a mechanical trigger),
the briefing ends by asking the commander three questions — what they mainly work
on, how they want status reported, and any quiet hours — and HQ writes the
answers into `LOCAL.md`, acting as the user's scribe into the user's own slot.
An edited `LOCAL.md` skips the step entirely; the questions are asked once, not
per rotation.

The trigger SHALL NOT be re-delivered when `gtmux hq` merely focuses an ALREADY-LIVE
supervisor (agent still running). It SHALL be best-effort and non-fatal (a delivery
that does not land SHALL NOT fail `gtmux hq`), bilingual (follows `GTMUX_LANG`), and
opt-out-able via `GTMUX_HQ_BRIEF` (`off`/`0`/`false`/`no`), defaulting on.

#### Scenario: A fresh spawn briefs on the first turn

- **WHEN** `gtmux hq` spawns a new supervisor session and the agent comes up
- **THEN** a single `» gtmux·startup` trigger is delivered into its pane, and the
  supervisor's first output — per its playbook's "First turn" section — introduces
  itself and reports the fleet status (needs-you first, token usage + subscription room)

#### Scenario: A first launch asks who the user is

- **WHEN** the briefing runs while `LOCAL.md` is still the untouched seeded template
- **THEN** the playbook directs HQ to close the briefing with the three
  personalization questions and to write the commander's answers into `LOCAL.md`

#### Scenario: A personalized home is not re-interviewed

- **WHEN** the briefing runs and `LOCAL.md` already carries user content
- **THEN** no personalization questions are asked

#### Scenario: A relaunched dead pane briefs too

- **WHEN** `gtmux hq` relaunches the agent in a stamped-but-dead HQ pane
- **THEN** the same `» gtmux·startup` trigger is delivered, so the revived supervisor
  briefs just like a fresh spawn

#### Scenario: A focused live supervisor is not re-briefed

- **WHEN** `gtmux hq` runs while a supervisor session is already live (agent running)
- **THEN** it focuses the existing session and NO startup trigger is delivered

#### Scenario: Opt-out spawns HQ silently

- **WHEN** `GTMUX_HQ_BRIEF` is `off`/`0`/`false`/`no` and `gtmux hq` fresh-spawns
- **THEN** no startup briefing is delivered — the supervisor waits at its prompt

#### Scenario: A briefing that cannot land does not fail the command

- **WHEN** the agent does not come up in time, or the delivery cannot be verified
- **THEN** `gtmux hq` still succeeds (the session is up and usable) rather than
  reporting failure
