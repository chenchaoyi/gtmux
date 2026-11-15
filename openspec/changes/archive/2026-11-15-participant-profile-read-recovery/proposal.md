# Recover participant process reads without inventing message facts

## Why
Some real HQ transcript replies carry a source session and agent but no message timestamp. Historical facts correctly stay unavailable, yet current process matching incorrectly depends on that unavailable historical response. Retry repeats the same false mismatch. Profile reads also lack a whole-read deadline.

## What changes
Use the explicitly recorded source agent as the fallback only when no historical profile identity exists. Keep exact source-session, server, home, agent and current/historical guards. Distinguish absent message facts, unavailable current reads and unknown identity. Bound read and body parsing to 15 seconds, retire late responses, and make retry a visible accessible action with immediate busy feedback. No historical timestamp, model or identity is fabricated.

## Surfaces
- Terminal (incl. attach): no change; no terminal input or lifecycle action.
- Menubar: no participant reader; no change.
- Phone: repair the shared participant modal and lazy process reader.
- iPad: same shared components and read scope.
- Web: existing source-checked lazy process reader remains; this change fixes the App-only dependency on an absent primary response. No Browser permission or endpoint change.

## Validation
Real shared React components with isolated deferred reads, exact agent/session mismatches, unsupported identity, deadline, retry and late-response boundaries. Existing profile tests retained. No device, real message, restart or live state repair acceptance is claimed.
