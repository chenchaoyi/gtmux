# restore-liveness-gate — restore resumes what was RUNNING, not what once ran here

## Why

After every reboot, `gtmux restore` conjured agent sessions into panes that had none.
On 2026-08-04 the fleet came back as **16 agent panes from 10**. The reported one:

> A pane in session `日常更新`, window `0:Daily Binary Update`, doing routine binary
> upgrades in a plain shell (opencode 1.18.9 → 1.18.11, `claude update`). After the
> reboot gtmux typed
> `{ cd -- '/Users/x' …; } && claude --resume '369b78a0-…'` into it, and it sat at
> Claude's trust gate. *"This pane had no cc session before the restore and does after.
> This has been happening all along."*

It is not cosmetic. The six phantoms burned subscription quota (one carried **33.7M
tokens** of a four-day-old conversation and tripped gtmux's own `usage·warn burn`
alarm), inflated the resident-session count on a 24GB machine that has already had a
session-count-driven kernel panic, and polluted HQ's model of the fleet — it opened
dossiers on ships that were not sailing.

**Root cause: restore asked the wrong question.** A resume record is written by the
agent's hooks and never pruned, so it says *"an agent ran in this pane at some point"* —
a permanent property of a locator. Restore read that as *"an agent was running here"*
and relaunched the conversation. Every pane that had ever hosted an agent was therefore
a candidate, forever. And it repeats because it feeds itself: the injected session
re-writes the very record — and, at the next autosave, the very `pane_full_command` —
that justifies the next injection. (`日常更新:0.0` was hit on 2026-07-27 and again on
2026-08-04, from a record whose timestamp the *previous* injection had refreshed. Any
age-based heuristic would have looked at a fresh record and waved it through.)

The evidence restore needed was already on disk. **Each `pane` line of a tmux-resurrect
save records that pane's `pane_current_command` and `pane_full_command` at save time.**
The processes are long dead when restore runs; the save is the only witness. In the
save taken 2.5 hours before that reboot, all six phantom panes read `bash`.

## What changes

Two questions, two sources, kept apart:

- **"Was an agent alive in this pane?"** — the SAVE. A pane the save shows at an
  interactive shell is never typed into, whatever its resume record remembers. A pane
  absent from the save is not restore's to touch either.
- **"Which conversation?"** — the resume RECORD at that locator, which the hooks keep
  current (it follows `/clear` and compaction; the save's command line only knows the
  id the process was launched with). New: when a pane with agent evidence has *no*
  record, the id is read back out of the save's own `claude --resume <id>` — so a
  missing record costs a pane a stale label, not its conversation.

Two supporting changes fall out of it:

- **The CWD fallback is now a last resort under the gate**, and refuses records nobody
  has touched in the fortnight before the save. It is a guess by construction — the
  record it finds belonged to a *different* locator — and the store accumulates ghosts
  (117 records on the reporting machine, back to sessions named `61` and `12`).
- **`restore --plan` (and the menu bar's expandable restore row) applies the same
  gate.** The plan is a promise about what restore will do; it was advertising the
  phantoms too.

### The parsing trap this uncovered (do not "simplify" it away)

tmux-resurrect's `save.sh` re-reads its own dump with `while IFS=$d read …`. The
delimiter is a TAB — and a tab is IFS *whitespace*, which bash collapses. **A pane with
an EMPTY title loses its field entirely and every later field shifts left by one**, so
those lines carry the pane's *pid* where the command belongs, plus a full-command field
resurrect computed from the wrong pid. Four of the six phantoms sat on shifted lines,
including the reported one: read at the fixed column, `日常更新:0.0` says its command is
`77304`, which is not a shell, which sails through the gate. The parser detects the
shift by the `:` that the format prefixes onto the directory (index 7 normally, index 6
when the title collapsed) and drops the untrustworthy full command on those lines.

## Impact

- Affected specs: `session-restore`
- Affected code: `internal/app/restoresave.go` (new), `internal/app/agent_resume.go`,
  `internal/app/restore_plan.go`, `internal/resume/fromcommand.go` (new),
  `internal/agents/accessors.go` (`CommandKeys`)
- Behavior: restore resumes strictly fewer conversations — only the ones that were
  live. Measured on the reporting machine's real pre-reboot save: **16 → 10**, and the
  10 are exactly the panes whose saved command line was an agent.
- Not addressed here (follow-ups): the resume store is still never pruned (the gate
  makes ghosts harmless, not absent); the same empty-title shift also corrupts the
  DIRECTORY resurrect restores those panes to (they come back at `/`), which is
  upstream behavior gtmux only reads around.
