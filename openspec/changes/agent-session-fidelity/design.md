# Design: agent-session-fidelity

## Correlation order

1. Read the agent session id and cwd from the hook payload.
2. If the session id matches exactly one current resume binding whose pane is a live Codex
   pane and whose cwd agrees, use that pane.
3. Otherwise accept the inherited pane only if it is the sole live Codex pane for that cwd.
4. Otherwise accept a cwd match only if exactly one live Codex pane has that cwd.
5. On ambiguity, do not write pane state, resume bindings, or receipts for a guessed pane.

This keeps the fast path for a correctly inherited pane while preventing a shared Codex
app-server from assigning events by directory alone. A matching resume binding is evidence
only when the live pane and cwd also agree; stale records cannot claim a pane.

## Session lineage

Add optional `agent` and predecessor-agent metadata to HQ session replacement audit records.
The textual summary remains unchanged for old readers. New readers prefer structured fields
and fall back to parsing the legacy summary plus `AgentForSession`. The session key stays
agent-local; callers must not treat a session id as a globally unique identity.

The same identity rule applies to incremental usage counters: persist them under the pair
`agent key + session id`. On first read, fall back to the legacy session-only counter and
write the next update under the new key. New counters for different agents can no longer
collide when an agent uses a non-UUID session id.

## Agent capability conformance

Keep the manifest as pure identity/capability data. Add checks that every non-empty transcript
key resolves to a parser, every `Hooked` entry has an installer and display mapping, and every
`Semantics` entry has a classifier table. Do not move event schemas or parser functions into
the registry.

## Codex parser fixtures

Fixtures cover `event_msg` prompts and replies, `response_item` user input, injected repository
instructions, tool calls, and unknown records. Expectations describe the normalized turns,
not a byte-for-byte copy of Codex's internal schema. Fixtures contain no user secrets.

## Codex plan metadata

Add an optional `plan_type` field to the limits window model and set it from the same
`rate_limits` record that supplies each Codex window. Missing/unknown values remain empty.
Existing renderers and clients remain compatible because the field is additive.

## Risks and fallback

- When no explicit Codex session binding exists, same-cwd panes may temporarily lose precise
  pane-scoped hook updates. The event is not misfiled; transcript/limits data remains readable.
- Legacy HQ audit entries have no structured agent identity. They continue to use the current
  unique-log inference, returning unknown on ambiguity.
- Codex may change its local JSONL schema. Unknown records are ignored and fixtures make
  supported shapes explicit; they do not claim all future versions are covered.
