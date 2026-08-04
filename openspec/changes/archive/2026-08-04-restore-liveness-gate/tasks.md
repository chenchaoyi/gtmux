# Tasks — restore-liveness-gate

## 1. Read the save as evidence

- [x] 1.1 `internal/app/restoresave.go`: parse a resurrect save's `pane` lines into
      `savedPane{Loc, Dir, Cmd, Full, Shifted}`, handling BOTH field layouts (the
      empty-title shift) and un-escaping the directory.
- [x] 1.2 `paneEvidence` (`missing|shell|other|agent|unclear`) + `allowsResume()`:
      deny shell/other/missing, allow agent/unclear (ambiguity must not cost a live
      conversation).
- [x] 1.3 Tests over REAL save lines from the incident — normal, shifted, `:`-prefixed
      title, escaped spaces, malformed; every phantom pane classified `shell`.

## 2. Identify an agent from a dead command line

- [x] 2.1 `internal/agents`: `CommandKeys()` — executable name → agent key, from the
      registry (identity stays in the SSOT).
- [x] 2.2 `internal/resume/fromcommand.go`: `FromCommand(cmdline) (agent, sessionID)` —
      the inverse of `Command()`, driven by the same `ResumeArgv` table; skips
      interpreters and env prefixes.
- [x] 2.3 Tests: per-agent flag shapes (`--resume`, `resume`, `--session`, `-r`),
      `--flag=value`, absolute paths, `node …/claude`, non-agents; round-trip against
      `Command()`.

## 3. Gate the resume

- [x] 3.1 `resumeAgents`: build the layout once, drop panes whose evidence forbids a
      resume, log each denial with the saved command; no save ⇒ no gate (degrade to the
      old behavior rather than to silence).
- [x] 3.2 Add the save-derived session id as the second source, after the exact record
      and before the CWD guess.
- [x] 3.3 CWD fallback: prefer the SAVE's directory (the live one is `/` when the
      shifted line broke the restore) and refuse records older than 14 days before the
      save (`fallbackMaxAge`).
- [x] 3.4 Tests: age belt (fresh recovered, month-old refused, undated not punished).

## 4. The plan must promise the same thing

- [x] 4.1 `buildRestorePlanFrom`: walk the save, apply the same gate and the same
      source order.
- [x] 4.2 Tests: a shell pane with a record is not listed; an agent pane with no record
      recovers its id from the saved command line.

## 5. Docs

- [x] 5.1 `docs/cli.md`: what restore brings back, in one paragraph under `gtmux restore`.
- [x] 5.2 `docs/TROUBLESHOOTING.md`: the empty-title field shift (symptom → cause →
      must-check), per the repo's footgun rule.
- [x] 5.3 Spec synced into `openspec/specs/session-restore/spec.md`; change archived.
