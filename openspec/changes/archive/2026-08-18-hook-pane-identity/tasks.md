# Tasks — hook-pane-identity

> **Delivery context (2026-10-06 audit).** The completed 2.1/2.2 refer to the
> [delivered calibration](proposal.md#hook-pane-identity--a-hook-proves-which-pane-it-came-from),
> which compares a bound log with recent unclaimed conversations in its directory,
> rather than tmux window activity. Original task text and completion marks are
> retained below as history; they are not fresh cross-agent acceptance evidence.

## 1. Identity by evidence (the root cause)
- [x] 1.1 `internal/hook`: resolve the pane from process ancestry when `$TMUX_PANE` is absent — match `pane_pid`, then `pane_tty`; bounded walk; ambiguity → not identified
- [x] 1.2 Wire it in ahead of the native-session branch, so state/resume/events all get the pane
- [x] 1.3 Tests: pid match, tty match, no match (stays native), ambiguous (stays native), bounded walk

## 2. A stale binding says so
- [x] 2.1 `gtmux doctor` row: pane active while its bound transcript is silent
- [x] 2.2 Tests: stale reported with pane id + age; a quiet pane is not reported

## 3. The chat refreshes on its own
- [x] 3.1 `GET /api/transcript`: ETag + `304` on an unchanged log
- [x] 3.2 Mobile: poll while the chat view is open, conditional on the ETag
- [x] 3.3 Tests: server 304 path; the app refetches without a status flip

## 4. Record the problem
- [x] 4.1 `docs/TROUBLESHOOTING.md` entry: symptom → root cause → must-check
- [x] 4.2 Memory note
- [x] 4.3 Sync specs + archive this change
