# Recover verified Codex binding after a transient observation failure

## Why
A successful root-thread response can fail its strict terminal recheck once, after which the connection drops the candidate and an empty attachment never retries. The TUI keeps working while App conversation association remains unavailable.

## What Changes
Retain the verified per-connection root, retry only a transient publication failure within a short bounded schedule, and invalidate pending work on a newer root selection or connection epoch. Preserve strict identity and separate unknown/stale history from control authority.

## Surfaces
- terminal: native conversation continues; no replayed input, reconnect, restore or new root RPC.
- menubar: consumes the existing correctly associated radar row after confirmed publication.
- phone: existing conversation/input guards remain; no fabricated association.
- iPad: same shared consumer semantics as phone.
- Web: same server identity; source-only recovery is not installed UI acceptance.

## Impact
Codex bridge connection state and typed frontend validation only; no access-policy, credential, TCC or real-session mutation during tests.
