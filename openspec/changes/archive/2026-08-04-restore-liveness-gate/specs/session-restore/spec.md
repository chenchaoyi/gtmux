# session-restore (delta)

## ADDED Requirements

### Requirement: Resume only panes that were running an agent when the layout was saved

`gtmux restore` SHALL decide whether a restored pane gets an agent conversation from the
tmux-resurrect SAVE — the `pane_current_command` / `pane_full_command` it recorded for
that pane — and NOT from the presence of a resume record at the pane's locator.

A resume record is written by the agent's hooks and is never pruned, so it attests only
that an agent ran in that pane at SOME point; treating it as evidence of a live session
made every pane that had ever hosted an agent a permanent resume target. A pane the save
shows sitting at an interactive shell SHALL NOT be resumed into, and a pane with no line
in the save SHALL NOT be resumed into either (restore did not create it). When the save
records a program that is not an agent, the pane SHALL NOT be resumed into. When the save
records something that cannot be identified — no full command, and a
`pane_current_command` that names no known agent (a live Claude Code pane reports its
VERSION there) — the resume SHALL be allowed and the ambiguity logged: silently failing
to bring back a conversation that really was running is the worse failure.

When NO save can be read at all, the gate SHALL be disabled rather than deny every
resume, so a missing save degrades to the previous behavior and not to silence.

The parser SHALL handle both of tmux-resurrect's pane-line layouts. resurrect re-reads its
own dump with a tab-delimited shell `read`, and because a tab is IFS whitespace a pane with
an EMPTY title loses that field, shifting every later field left by one; such lines carry
the pane's pid where the command belongs and a full command computed from the wrong pid.
The directory field's `:` prefix distinguishes the layouts, and the full command on a
shifted line SHALL be discarded.

#### Scenario: A shell pane is left alone

- **WHEN** a restored pane's saved line shows an interactive shell, and a resume record
  exists at that pane's locator from an agent that ran there in the past
- **THEN** restore types nothing into that pane and logs that the save shows a shell

#### Scenario: A live agent pane comes back

- **WHEN** a restored pane's saved line shows an agent command (`claude --resume <id>`)
- **THEN** restore resumes that pane's conversation

#### Scenario: An empty pane title does not disguise a shell

- **WHEN** a saved pane line lost its title field (fields shifted left), so the fixed
  command column holds the pane's pid
- **THEN** the parser reads the shifted layout and still sees the pane's real command,
  so a shell pane on such a line is refused like any other

#### Scenario: Unidentifiable but running

- **WHEN** a saved pane line records no usable full command and a
  `pane_current_command` that matches no known agent
- **THEN** restore allows the resume and logs that the evidence was unclear

### Requirement: Recover the conversation id from the saved command line

When a pane the save shows running an agent has NO resume record at its locator,
`gtmux restore` SHALL take the conversation id from the agent command line the save
recorded for that pane (e.g. `claude --resume <id>`), before falling back to any
directory-based guess. The resume record SHALL still take precedence when present: hooks
keep it current across `/clear` and compaction, whereas the saved command line only knows
the id the process was launched with.

#### Scenario: Missing record, id in the save

- **WHEN** a restored pane was running `claude --resume <id>` at save time and no resume
  record exists for its locator
- **THEN** restore resumes `<id>` into it rather than leaving the pane empty

#### Scenario: The record disagrees with the saved command line

- **WHEN** both a resume record and a saved command-line id exist for the same pane
- **THEN** the record wins

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
were never running. Position agreement is the evidence that the restored pane is the same
pane that hosted the conversation (its session having only been renamed); a pane at a
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

- **WHEN** a pane's exact locator no longer matches (its session was renamed) but a
  saved record shares the pane's directory and its window.pane position
- **THEN** restore recovers that conversation into the pane

#### Scenario: A long-abandoned record is not guessed into a pane

- **WHEN** the only record matching a pane's directory and position was last updated
  more than a fortnight before the save was taken
- **THEN** restore recovers nothing into that pane

### Requirement: The restore plan shows what restore will actually do

`gtmux restore --plan` (and its `--json` form, the menu bar's source for the expandable
restore row) SHALL list, per saved session, only the agent conversations that the real
restore would relaunch — applying the same liveness gate and the same
record → saved-command-line → directory-guess order. A conversation the plan lists but
restore would not resume (or the reverse) is a defect: the plan is a promise about what
the restore is about to do.

#### Scenario: The plan does not advertise a phantom

- **WHEN** a saved pane was an interactive shell but a resume record exists at its locator
- **THEN** the plan lists no conversation for that pane

#### Scenario: The plan counts the same conversations restore resumes

- **WHEN** a plan is built from a save and a restore is then driven from that same save
- **THEN** the conversations listed and the conversations resumed are the same set
