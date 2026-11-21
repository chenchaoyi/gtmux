## Why

A conflict between two incarnations of one keyed reminder can prevent unrelated HQ wakes from draining. A full queue can then discard deadline wakes and urgent work while the existing degradation marker remains set. The operator needs observable delivery stages and safe recovery without replaying uncertain input.

## What Changes

- Give each new queued wake a unique incarnation and protect only exact conflicting slots when recovery can prove their scope.
- Bound retries of known-unsent attempts and preserve possibly submitted input for reconciliation.
- Compare all candidates at capacity, retain deadline debt in the workspace, and report delivery stages and failures through durable diagnostics and existing owner notifications.
- Exercise rotation identity changes independently of automatic wake delivery.

## Surfaces

- CLI and resident service share the wake queue, diagnostics and workspace records.
- The macOS menu bar presents the existing owner notification channel.
- iOS and Android continue to consume workspace state through the existing protocol.
- The browser reads the existing workspace state and keeps the same authorized access.
