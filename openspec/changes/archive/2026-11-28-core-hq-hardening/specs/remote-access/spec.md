## ADDED Requirements

### Requirement: Non-blocking hub loop, conditional transcripts, and remote scope hardening

`gtmux serve` SHALL run `onSlowTick` in a single-flight background goroutine followed by `checkServerMode`, re-verify enrolled and guest bearer tokens before writing SSE events, restrict `GET /api/addresses` to full-scope callers, strip `cwd`, `icon`, and `launch_host` from guest `/api/agents` responses, bound JSON `POST` bodies to 64 KiB, short-circuit unchanged `/api/transcript` `If-None-Match` polls, and mark/audit `/api/send` deliveries.

#### Scenario: Slow tick does not stall SSE or fast nudge ticks

- **WHEN** `onSlowTick` blocks on slow external sampling
- **THEN** radar diffs, fast nudge ticks, and SSE `ping` frames continue on schedule while overlapping slow ticks are skipped

#### Scenario: Revoked token or guest boundary is enforced live

- **WHEN** an enrolled device or guest link is revoked or expires while `/api/events` is open, or a guest requests `/api/addresses` or `/api/agents`
- **THEN** `/api/events` closes on the next event, `/api/addresses` returns `403`, and guest `/api/agents` omits host paths and icons
