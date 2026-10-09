## ADDED Requirements
### Requirement: Restore fallback preserves conversation ownership
Before resuming any shell pane, the system SHALL reserve conversation IDs bound to live non-shell panes. A cwd/position fallback SHALL NOT select a record whose original locator still exists in the saved layout or live topology. It SHALL accept only one distinct unreserved conversation and SHALL refuse multiple candidates rather than choosing by recency. A skipped fallback SHALL be reported to the user and diagnostics; the restore plan SHALL use the same locator and ambiguity rules. Exact and saved-command matches SHALL retain priority over guesses.

#### Scenario: Another client already owns the conversation
- **WHEN** a restored shell shares cwd and position with a running pane's recorded conversation
- **THEN** no resume command for that conversation is typed into the shell

#### Scenario: Two old conversations could match
- **WHEN** two distinct eligible conversations remain after ownership exclusions
- **THEN** restore skips the fallback and names its ambiguity instead of choosing the newest

#### Scenario: The original locator still exists
- **WHEN** a record belongs to another locator still in the saved layout, even if that pane is an idle shell
- **THEN** it cannot be interpreted as a renamed session

#### Scenario: A unique rename remains recoverable
- **WHEN** exactly one fresh candidate shares cwd and position, its original locator is absent, and its conversation is not reserved
- **THEN** both plan and restore select that conversation

## MODIFIED Requirements
### Requirement: The cwd fallback requires layout-position agreement

When a restored pane's exact locator has no saved resume record and the save recorded no
conversation id for it, `gtmux restore` MAY recover a conversation by a fallback, but ONLY
from a record that shares BOTH the pane's working directory AND its window.pane layout
position (the coordinates tmux-resurrect preserves across a reboot). A directory match
alone SHALL NOT authorize a resume. This is required because many restored panes are plain
shells (editors, extra terminals) that merely sit inside a project directory without ever
having hosted an agent; a directory-only fallback injected a historical conversation into
every such pane, so a single session came back showing several agent conversations that
were never running. Position agreement is necessary but not sufficient: ownership and uniqueness SHALL also
be checked as required by Restore fallback preserves conversation ownership. A pane at a
position no agent ever occupied SHALL recover nothing.

The fallback runs only for panes that already passed the liveness gate above, and because
it is a GUESS — the record it matches was saved under a DIFFERENT locator — it SHALL
additionally refuse records that were last updated more than a fortnight before the save.
The directory compared SHALL be the one the SAVE recorded for the pane when available,
which is the pre-reboot truth; the pane's live working directory is `/` whenever the
directory failed to restore.

#### Scenario: A bare shell pane sharing a project directory is not injected

- **WHEN** a restored pane at a window.pane position that never hosted an agent sits
  in a directory where some other pane once ran a conversation
- **THEN** restore resumes nothing into it, rather than injecting a historical
  conversation from that directory

#### Scenario: A renamed session still resumes at its position

- **WHEN** a pane's exact locator no longer matches (its session was renamed) but exactly one eligible
  conversation remains at an absent original locator, sharing directory and position
- **THEN** restore recovers that conversation into the pane

#### Scenario: A long-abandoned record is not guessed into a pane

- **WHEN** the only record matching a pane's directory and position was last updated
  more than a fortnight before the save was taken
- **THEN** restore recovers nothing into that pane

