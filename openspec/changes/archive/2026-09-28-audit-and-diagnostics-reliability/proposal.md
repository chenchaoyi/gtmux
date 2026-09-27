# audit-and-diagnostics-reliability

## Why

A repository and live-log audit found two observable failures. On an unchanged tmux
layout, tmux-resurrect exits successfully but leaves its `last` pointer untouched;
gtmux interprets the old mtime as a failed save and runs `save.sh` again on every
20-second serve tick. On the audit machine this produced about 26,000 restore entries
in seven days. Separately, HQ's `last-distill` and `last-self-check` markers are
written when a maintenance wake is **raised**, yet doctor and the capture banner say
the passes **ran**. A missed or ignored wake can therefore appear healthy.

The same audit found mobile controls that speak test identifiers to VoiceOver,
low-contrast connection-page help, and browser interactions with no executed UI
regression coverage. The old test plan and menu-bar design references no longer name
the current product and code.

## What changes

- Bound restore backstop attempts separately from the last *changed* snapshot, and
  retry failed attempts at a shorter but bounded interval.
- Give HQ maintenance an explicit completion receipt. Keep the trigger timestamp
  for cadence, but show a pending pass until HQ acknowledges its work. Record the
  receipt in the event journal and diagnostic log with structured fields.
- Keep mobile test IDs as identifiers and give controls localized spoken labels;
  improve connection-page text contrast and shorten pairing guidance.
- Add an executed Web interaction regression for the highest-risk guest and composer
  flows, while keeping the Go API tests.
- Correct current design and test docs in both languages where paired, and close the
  already-implemented `agent-session-fidelity` change after verifying its gate.

## Surfaces

- **terminal / 终端** — restore saves stop looping; `gtmux hq` gains a maintenance receipt;
  doctor and capture distinguish a raised trigger from completed work.
- **menubar / 菜单栏** — consumes the same honest diagnostics; its documented code paths are
  corrected, with no new control.
- **Phone** — screen-reader labels and connection guidance improve; diagnostics keep
  the existing local buffer and Mac-side action trail.
- **iPad** — shares the same React Native controls and copy as phone.
- **Web** — behavior stays the same; browser regression coverage is added.

## Non-goals

- Do not edit the operator's private knowledge entries automatically. Lint findings
  that require factual or sensitivity judgment remain a reviewed queue.
- Do not deploy Workers, alter production credentials or configuration, or cut a release.
