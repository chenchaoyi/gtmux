## Why

A conflict between two incarnations of one keyed reminder can prevent unrelated HQ wakes from draining. A full queue can then discard deadline wakes and urgent work while the existing degradation marker remains set. The operator needs observable delivery stages and safe recovery without replaying uncertain input.

## What Changes

- Give each new queued wake a unique incarnation and protect only exact conflicting slots when recovery can prove their scope.
- Bound retries of known-unsent attempts and preserve possibly submitted input for reconciliation.
- Compare all candidates at capacity, retain deadline debt in the workspace, and report delivery stages and failures through durable diagnostics and existing owner notifications.
- Exercise rotation identity changes independently of automatic wake delivery.

## Surfaces

- The terminal CLI and resident service share queue admission, diagnostics and workspace records.
- The macOS menubar presents independent owner notifications through its existing queue.
- The phone app reads the existing workspace state and can show owed matters.
- The iPad app uses the same workspace contract and access policy.
- The Web workbench reads the existing workspace records under its existing access rules.
