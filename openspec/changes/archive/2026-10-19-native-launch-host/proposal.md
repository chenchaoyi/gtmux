# Native launch host observations

## Why

Creation metadata identifies an agent protocol, not the application hosting its conversation. A native Codex row currently says “Client unconfirmed” even when its launch ancestry can supply a useful source name. An open source integration must derive that name generically, without private wrapper names.

## What changes

Observe a bounded parent chain on session identity hooks, before Codex detaches its hook worker. Store a comparable historical evidence value and expose only an optional display name, kind and observation time. Prefer an enclosing macOS app display name; otherwise use the executable basename. Stop at interpreters or service boundaries. All consumers keep the harness identity and existing authority checks.

This does not replay launch commands, change restoration, infer pane binding or current input ownership, grant follow/send/adopt authority, or backfill old sessions from a machine-wide process scan. The older wrapper-launched-agents proposal concerns restore and is outside this change.

## Surfaces

- terminal: native row location shows the observed host, or the neutral external-session fallback for unconfirmed clients; JSON carries optional display metadata.
- menubar: the native subtitle shows agent and host; read-only conversation details show the observation time and current input uncertainty.
- Phone: the shared native row and conversation details consume the same metadata.
- iPad: the same shared row/detail implementation as the phone; no separate detection path.
- Web: native rows and conversation details use the core observation, with escaped text and timestamp.

## Risks

Process names are mutable display hints. Ancestry can disappear, be reparented, exceed the budget or end at a generic runtime; those cases remain unknown. A historical observation is never proof of the current controller or liveness. No full command arguments or executable paths cross the API.
