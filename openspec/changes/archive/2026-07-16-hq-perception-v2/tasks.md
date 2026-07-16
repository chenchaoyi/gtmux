# Tasks: hq-perception-v2

## 1. Wake channel core (internal/hqwake, refactor of hook nudge call sites)

- [x] 1.1 New `internal/hqwake`: wake-line builder (`» gtmux·<class> │ …` columnar
      format, DATA quoting), class enum, and fixture tests pinning the format
      (incl. a POSIX/C-locale robustness fixture — the ✳/LANG regression class).
- [x] 1.2 Route ALL existing HQ injections (waiting / asks / resolved-with-chase /
      goal-changed→`user-driving` / reap-suggest / feed-degraded) through hqwake;
      delete `feedSupersedesReceipts` and the receipt-suppression branches in
      `internal/hook/nudge.go`.
- [x] 1.3 Consumer-freshness (spool consumed-cursor age) chooses compact vs
      self-contained wake payload — never suppression. Unit test both forms.
- [x] 1.4 `done` wake for ANY session: on Stop reaching idle, fire unless the pane
      is the focused pane of an attached client (`#{pane_active}` + attached);
      attended completions increment the tick tally. Config `hqWake.done =
      unattended|always|tick`. Payload: loc │ duration │ goal │ tail. Tests:
      unattended fires, attended tallies, config overrides.
- [x] 1.5 Per-pane merge window (default 120s, `hqWake.paneMinGapSec`): a newer
      done for the same pane replaces the queued line. Test the flood case
      (5 dones → 1 line).
- [x] 1.6 `new-session` wake on first sight of an agent pane (SessionStart or
      first radar classification), deduped per pane. Test.

## 2. Tick scheduler (serve slow-tick)

- [x] 2.1 Outcome tally (done/new/gone/stall-suspect) persisted beside the events
      cursor; tick delivery in the serve slow-tick: interval (default 10m,
      `hqWake.tickMinutes`) + burst threshold (default 5, `hqWake.tickBurst`);
      ZERO tally → no injection (test: quiet hour injects nothing).
- [x] 2.2 Tick wake line carries seq range + counts; delivered via hqnudge
      (draft-guarded). Test early-fire on burst.

## 3. Crash sensing + delta read

- [x] 3.1 Hook: handle `StopFailure` → `crash` event (severity important, error
      head as DATA summary), no normal finished stamp; immediate `crash` wake.
      Install-hooks wiring for the new event. Tests: classifier + severity +
      no-finish.
- [x] 3.2 `gtmux events --since-seq <n>` one-shot delta read (combinable with
      --severity/--json). Tests. Document in docs/cli.md + help.go (en+zh).

## 4. Playbook v2 + legacy migration

- [x] 4.1 `hqInstructions` v2: wake-protocol semantics (wake→pull→judge, short
      turns), enrollment protocol (dossiers: purpose/status/channel; one
      transcript-head drill max), signal register (⟣ ✅/▪/◈/⚠ vocabulary, tick
      brief ≤6 lines), graded done judgment, drop the background `hq-feed --tail`
      requirement (pull-on-wake), keep tier gating/ledger/quiet. Bump
      `hqPlaybookVersion` to 2.
- [x] 4.2 `seedHQHome`: migrate the legacy CLAUDE.md-only home (timestamped
      backup + managed AGENTS.md + pointer + LOCAL.md + printed notice). Tests:
      legacy → migrated, managed → upgraded, fresh → seeded, user CLAUDE.md
      content preserved in backup.

## 5. Consistency + verification

- [x] 5.1 Reconcile specs: fold deltas into `openspec/specs/{hq-wake-protocol,
      session-events,supervisor-agent}`; `openspec validate --specs --strict`
      green.
- [x] 5.2 Update CLAUDE.md HQ paragraph + `docs/cli.md` (`events --since-seq`);
      memory notes stay accurate (hq-attention-system description now historic).
- [x] 5.3 `make check` green; CGO_ENABLED=0 build green.
- [ ] 5.4 (post-merge) Dogfood on the commander's machine: migrate the live legacy home, run a
      busy hour, verify: HQ screen shows only wake lines + signal replies; an
      unattended user-direct finish surfaces within seconds; an attended chat
      session does not wake HQ per reply; a quiet hour injects nothing.
