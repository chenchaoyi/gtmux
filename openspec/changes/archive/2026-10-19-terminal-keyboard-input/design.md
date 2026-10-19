# Design

The design and implementation record lives in the bilingual
[research and interaction document](../../../../docs/design/terminal-keyboard-research.md).
It is implemented; author validation and remaining acceptance are recorded in tasks.md.
No independent review is claimed.

## Decisions

1. Keep the existing frequent controls and add one visible Keys entry beside the
   keyboard toggle. Do not turn the toolbar into a full keyboard.
2. The panel groups navigation, function and character keys. Ctrl/Alt/Shift are
   one-shot selections, cleared after a target tap and all target/lifecycle changes.
3. A cgo-free `internal/terminalkeys` leaf owns normalization, valid modifier sets,
   aliases, conservative combination support and encoding. Surfaces own presentation only.
4. Provide pane-scoped `/api/input-capabilities` and additive `key_event`,
   `input_context` and contextual `action` forms on `/api/send`. New event forms
   require `send_id`; a single request cannot mix event, action, legacy key or text.
5. Validation precedes copy-mode exit and writes. Scope/consent/grants are checked
   for new sends and duplicate receipts. Stale target/context is a refusal.
6. Deduplicate new events by caller, server boot, pane binding and payload, including
   concurrent requests. Receipts are bounded; never replay an uncertain event after
   a boot change or outside the retained retry horizon. Delivery is not execution.
7. A semantic action is resolved by the server with an ID and revision. Unknown or
   custom Codex bindings suppress the action; raw Shift+Left stays a generic key.
8. Hook/structured evidence comes first where actually exposed; a narrow live-footer
   fallback can be implemented centrally. No second app-server or resume for discovery.

## Tradeoffs

- One extra visible toolbar entry costs width but is discoverable without hidden
  gestures or a nested More menu. Existing horizontal scrolling remains available.
- One-shot modifiers avoid hidden persistent input state. Repeated Emacs-style
  sequences cost additional taps; locking/macros remain future work.
- A versioned catalog adds one capability fetch but avoids shipping client/server
  key lists that disagree. Cache by boot/version rather than polling it every frame.
- Legacy terminal encodings cannot distinguish every modified character. Refusing
  unsupported combinations is preferable to silently sending a different event.
- Bounded receipt storage improves safe retries but does not establish exactly-once
  delivery across crashes. Unknown outcomes remain visible and are not auto-replayed.

## Implementation checkpoints

Determine the repository's minimum supported tmux versions on macOS and Linux,
test the advertised matrix there, and document unsupported pane modes. The tmux
3.7c byte experiment is evidence for only that version and its default app mode.
Confirm which deployed Codex hook or read-only event source actually exposes pending
questions and custom bindings. Its Rust UI state alone is not an external API.

The implemented baseline and hook-source investigation are recorded in tasks.md.
Remaining device/live-agent acceptance is explicit and is not a claimed pass.
