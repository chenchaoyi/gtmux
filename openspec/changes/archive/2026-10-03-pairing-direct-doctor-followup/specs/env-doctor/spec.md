## MODIFIED Requirements

### Requirement: Read-only grouped health check

The system SHALL, on `gtmux doctor`, run a read-only check grouped by concern
(tmux, restore, terminal, agents+notifications), each row a status glyph + label
+ value + a short "why", and a summary tally. The check itself SHALL change
nothing. When it finds improvable or blocking rows AND is running on an
interactive terminal (a TTY), it SHALL, after the report, OFFER to apply the
fixes inline (the same consent-gated per-step flow as `--fix`), so the user does
not have to re-invoke with `--fix`; declining the offer, or running off a TTY,
keeps the command read-only and prints the `--fix` hint instead.

The system SHALL announce each check section on stderr before evaluating that
section when running in a terminal. `--progress` SHALL enable the same announcements
when output is redirected. The optional Homebrew update suggestion SHALL NOT delay
the report indefinitely.

During the agent and notification section, `gtmux doctor` SHALL announce each
subcheck before running it when progress is enabled. The last line SHALL name
the currently running hook, chat-binding, or hook-traffic probe.

#### Scenario: Healthy environment

- **WHEN** `gtmux doctor` runs with everything configured
- **THEN** it prints the grouped checks all ✓ and exits 0, changing nothing

#### Scenario: Blocking issue

- **WHEN** a required prerequisite is missing (e.g. tmux absent, or set-titles
  not configured for focus/restore)
- **THEN** that row is marked blocking and the command exits non-zero

#### Scenario: Offer to fix inline on a TTY

- **WHEN** `gtmux doctor` (no `--fix`) finds improvable/blocking rows on an
  interactive terminal
- **THEN** after the report it asks whether to fix now, and on assent walks the
  same consent-gated fix flow; declining keeps it read-only

#### Scenario: Non-interactive stays read-only

- **WHEN** `gtmux doctor` runs off a TTY (piped / CI) with improvable rows
- **THEN** it does NOT prompt and changes nothing, printing the `gtmux doctor
  --fix` hint

#### Scenario: A check is slow

- **WHEN** `gtmux doctor` runs in a terminal, or with `--progress` when piped
- **THEN** it writes each check section to stderr before running that section,
  so the last visible stage identifies where it is waiting; stdout remains the
  grouped report
- **AND** the optional Homebrew update probe uses cached metadata without an
  automatic update and stops after a bounded wait, leaving the tmux version row
  available even if Homebrew is stuck

#### Scenario: A probe stalls

- **WHEN** a doctor probe is slow
- **THEN** the last announced section identifies the current stage
- **AND** a stuck Homebrew update lookup times out, allowing the report to finish

#### Scenario: Agent probe is slow

- **WHEN** a chat-binding or hook-traffic probe takes a long time
- **THEN** stderr identifies that probe without changing the report on stdout
