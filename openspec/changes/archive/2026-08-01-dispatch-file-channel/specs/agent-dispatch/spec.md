# agent-dispatch (delta)

## ADDED Requirements

### Requirement: Shell-free payload channel for dispatch text

The system SHALL accept a dispatch payload from a FILE or from standard input, so that
the payload never traverses a shell: `gtmux spawn --goal-file <path>` and `gtmux send
<pane> --message-file <path>`, each accepting `-` to mean standard input. The bytes read
SHALL reach the target agent's input box verbatim — backticks, `$`, single and double
quotes, interior newlines and multi-byte text included — with exactly ONE normalization,
which SHALL be specified rather than incidental: at most one trailing newline is
stripped (every heredoc and `printf` appends one). The positional argv form SHALL remain
supported for short instructions, and supplying BOTH a payload file and positional text
SHALL be rejected as an error rather than resolved by a silent precedence rule. An empty
payload SHALL be rejected. This exists because a payload passed as argv is necessarily
parsed by the caller's shell, and any sufficiently long natural-language instruction
eventually contains a character the shell assigns meaning to — so per-call caution
cannot be the guarantee.

#### Scenario: A goal containing shell metacharacters survives verbatim

- **WHEN** a goal containing backticks, `$`, double quotes and newlines is delivered via
  `gtmux spawn --goal-file <path>`
- **THEN** the text placed in the agent's input box is byte-for-byte the file's content
  (modulo at most one stripped trailing newline), and no part of it is evaluated

#### Scenario: Payload from standard input

- **WHEN** `gtmux spawn --goal-file -` or `gtmux send <pane> --message-file -` runs
- **THEN** the payload is read from standard input under the same byte-exact rule

#### Scenario: File and positional text together are refused

- **WHEN** `gtmux spawn --goal-file <path> some words` runs
- **THEN** the command exits with a usage error naming the conflict, and dispatches
  nothing

#### Scenario: An empty payload file is refused

- **WHEN** the referenced payload file is empty or holds only whitespace
- **THEN** the command exits with a usage error and dispatches nothing

### Requirement: The one-shot goal travels as bytes, not as a shell word

`gtmux spawn --oneshot` SHALL deliver its goal to the in-pane runner as FILE CONTENT
rather than as a shell-quoted word on the typed launch line. The runner
(`gtmux oneshot-run`) SHALL accept `--goal-file <path>` and read the goal from it. A
multi-line goal SHALL therefore survive a one-shot dispatch intact, where the typed-argv
form necessarily collapsed it to a single line.

#### Scenario: A multi-line one-shot goal keeps its newlines

- **WHEN** `gtmux spawn --oneshot --goal-file <path>` dispatches a goal containing
  newlines
- **THEN** the headless runner receives the goal with its newlines intact, not
  whitespace-collapsed onto one line

### Requirement: Re-entrant dispatch — a failed spawn converges on re-run

A `gtmux spawn` that fails partway SHALL NOT leave state that makes the identical command
fail differently the next time. The system SHALL guarantee this two ways, in order:

1. **Idempotent acquisition.** `AddWorktree` SHALL reuse a git worktree that already
   serves the requested branch instead of failing on the occupied path, reporting the
   reuse to its caller. A path occupied by a DIFFERENT branch SHALL remain a hard error —
   silently adopting an unrelated tree is worse than the failure it replaces.
2. **Resume, else rollback.** Before creating a session, spawn SHALL look for a previous
   attempt at the same dispatch — a ledger entry with `delivered:false` that owns its
   session, matches this invocation by worktree path (or by derived session name when no
   worktree is involved), and whose pane is still live — and ADOPT it: no second session
   is created, the agent is relaunched if the pane fell back to a bare shell, the goal is
   delivered, and the EXISTING ledger entry is updated rather than duplicated. When a
   step fails after this invocation created a worktree that cannot be resumed, spawn
   SHALL remove that worktree and delete the branch it created, so the failure leaves
   nothing behind.

#### Scenario: Re-running a failed worktree dispatch reuses the worktree

- **WHEN** `gtmux spawn --worktree <branch> …` is re-run after a prior attempt already
  created that worktree
- **THEN** the existing worktree is reused and reported as reused, instead of failing
  with git's "already exists" error

#### Scenario: Re-running adopts the undelivered session instead of creating a second

- **WHEN** a prior spawn created a session but never delivered its goal, and the identical
  command is re-run while that session's pane is still live
- **THEN** the existing pane is reused, the goal is delivered into it, and the ledger holds
  ONE entry for the dispatch — not a second empty session and a second entry

#### Scenario: A worktree created by a failed invocation is rolled back

- **WHEN** spawn creates a worktree and branch and then fails before a pane exists
- **THEN** the worktree it created is removed and the branch it created is deleted, so
  no half-built state survives the failure
