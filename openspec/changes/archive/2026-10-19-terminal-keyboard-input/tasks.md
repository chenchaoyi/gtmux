# Tasks and acceptance

## Implementation

- [x] Inspect the existing composer, grants, key path and retry boundary at a pinned baseline; record first-party terminal/Codex/tmux research in both languages.
- [x] Implement one cgo-free core catalog/encoder and a generated demo catalog; keep new combinations out of client hard-coded transport lists.
- [x] Add pane-scoped capability discovery and strict typed key/action requests; preserve valid legacy requests and old-server controls.
- [x] Retain current guest consent/per-link grants and recheck boot/live target before injection; reject malformed/unsupported/stale requests before terminal side effects.
- [x] Sign caller-bound contexts with a private secret separate from the public boot ID; bound receipt retention and deduplicate concurrent IDs and unconfirmed results.
- [x] Add a visible Keys entry, Navigation/Function/Characters groups and one-shot modifiers to phone/iPad/web; preserve drafts and attachments and display terminal-only receipts.
- [x] Pause later sends on uncertainty, keep exact explicit retry payloads and prevent replay under changed/expired context; clear modifiers on dismissal/background/permission/target changes.
- [x] Check Codex's pinned hook registry; implement a narrow current-route footer fallback with displayed keybinding and action revision revalidation, without a second controller or new status.
- [x] Cover core validation, scope, cache collision/capacity/concurrency, stale action, transport argv and all 407 advertised baseline byte combinations on an isolated tmux server.
- [x] Add mobile/web behavior tests and inspect rendered UI using a synthetic HTTP/auth server; record actual acceptance separately below.
- [x] Mutation-check protocol validation/dedup and client send serialization; run required repository gates and synchronize API/design/specs.
- [x] Archive the implemented change with its spec deltas.

## Acceptance

Baseline: tmux 3.7c on macOS, normal cursor/character modes, all 407 advertised
combinations checked against independent byte expectations. CI repeats this regression
on Linux's tmux; its actual result belongs in the PR, not an assumed pass here.
F1–F12, arrows, navigation/editing, classic character shortcuts and control aliases are
included. This does not establish compatibility with every older tmux or every agent's
keybindings/application modes.

The synthetic fixture exercises the real HTTP authentication, assets, catalog and
request handlers without reaching a production pane. Browser and iOS simulator checks
cover F2/Shift+F2 payloads, one-shot reset and receipts; browser checks also exercise a
contextual View question request and retained local draft. This is not a live Codex
question opening test. Native layout checks and final gate results are recorded in the
PR. Existing CI intentional skips are not passes.

Not verified: a real pending Codex question, physical-keyboard dispatch/IME on a device,
actual spoken VoiceOver, Android runtime, arbitrary custom bindings and every extended
terminal mode. No production release, installed-version change or access-policy change
is part of this implementation delivery. No independent agent/reviewer review was
performed; author tests and static checks are not independent review.

The footer fallback cannot distinguish a same-count replacement question or identical
footer-shaped output without native question IDs. Route inspection and tmux injection
are separate observations, not atomic control of the agent UI. It fails closed on
unrecognized hints and never claims exactly-once delivery across a crash.
