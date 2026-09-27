# agent-session-fidelity — preserve which agent session said what

## Why

gtmux combines pane state, hook events, transcript history, and plan windows from agents
whose identifiers and log formats differ. Codex hooks can run in a shared app-server whose
inherited `TMUX_PANE` is stale; cwd alone cannot distinguish two Codex sessions in the same
repository. HQ history stores session ids but currently infers the agent later from local
logs. These heuristics can misattribute an event or lose an agent label after a log moves.

Codex's current transcript also has multiple user-message representations. The parser has
adapted to `response_item` user input, but the schema comments and onboarding contract still
describe only the earlier `event_msg` shape. Codex plan data is available in the same rollout,
including `plan_type`, but only the windows currently reach the limits report.

## What changes

- Resolve a Codex hook to a pane by its explicit session id when possible. A cwd fallback is
  valid only when it identifies exactly one live Codex pane; ambiguity leaves the event
  unbound instead of writing to a potentially unrelated pane.
- Keep representative, sanitized Codex rollout fixtures and document the supported event
  shapes so parser changes can be checked against real logs.
- Persist agent keys with HQ successor/predecessor session links. Legacy records remain
  readable and continue to use conservative log-based inference.
- Key incremental usage counters by agent and session id, while migrating existing
  session-only counters on first read.
- Extend the agent registry conformance checks to ensure declared transcript and event
  capabilities are wired to implementations.
- Carry Codex's observed plan type as additive metadata on its limits windows, preserving
  existing output when the field is absent.

## Non-goals

- No agent-specific format is forced into a shared transcript parser.
- No lifecycle event is guessed when Codex session-to-pane correlation is ambiguous.
- No subscription endpoint is queried; Codex plan data remains sourced from local rollout
  records.

## User-visible behavior

Codex events will not be attached to a same-directory pane when gtmux cannot uniquely
identify the originating session. Earlier HQ turns retain their original agent label after a
tool switch. Codex limits may include its plan type when the rollout contains one.

## Surfaces

- **terminal** — `gtmux limits --json` includes optional `plan_type`; ordinary limits rows
  keep their existing wording.
- **menubar** — no visual change; its existing limits consumer ignores the additive field.
- **phone** — no visual change in this change; the typed API model accepts the additive field.
- **iPad** — same shared mobile API model as phone; no visual change.
- **web** — no visual change; existing clients may ignore the additive field.
