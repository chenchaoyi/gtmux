# hq-wake-protocol — delta

## ADDED Requirements

### Requirement: A cleared wait fires a resolved wake on any observed transition

A `resolved` wake SHALL fire whenever a pane transitions from `waiting` to
non-`waiting`, regardless of WHICH hook event (or no hook event) cleared the wait.
Emission SHALL NOT be gated solely on a specific set of hook events: a permission or
question approved in the pane's OWN window, after which the agent simply resumes,
often produces no prompt-submit / resumed / stop hook promptly (an agent may register
no post-tool hook, so the `waiting` marker persists until the turn's eventual stop),
yet the wait HAS cleared and HQ MUST be told so it can retract a stale needs-you.

The single writer (the serve slow-tick) SHALL observe each pane's `waiting` marker
and, on a `waiting → non-waiting` transition it detects, emit ONE `resolved` wake as
a backstop to the hook's event-specific emit. The transition state it tracks SHALL be
written only by that single writer (no read-modify-write race). The hook's immediate
`resolved` emit for the events it already covers SHALL remain as the fast path.

The hook fast path and the slow-tick backstop SHALL be DEDUPED so a single cleared
wait produces exactly ONE `resolved` wake, whichever channel observes the clear
first. The `resolved` wake SHALL be delivered on the acknowledged/retried/deduped
wake channel (the same channel as every other wake), NOT a bespoke best-effort send,
so it is retried on failure and escalates via the broken-channel path rather than
being silently dropped.

#### Scenario: Permission approved in the source window fires resolved

- **WHEN** a pane is `waiting·permission`, the user approves in that pane's own
  window, and the agent resumes with no resolving hook event
- **THEN** the slow-tick detects the `waiting → non-waiting` transition and fires one
  `resolved` wake so HQ retracts any pending chase about that pane

#### Scenario: A clear is announced exactly once

- **WHEN** both the hook fast path and the slow-tick backstop could observe the same
  cleared wait
- **THEN** only one `resolved` wake is emitted (the first observer wins; the other is
  deduped)

#### Scenario: resolved rides the acked channel

- **WHEN** a `resolved` wake's delivery to the HQ pane fails
- **THEN** it is retried on the acknowledged wake channel and, on repeated failure,
  surfaces via the broken-channel escalation — it is never silently lost
