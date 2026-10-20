# Compact secondary conversation information

## Why

A supplied phone screenshot shows a single Wait repeated across a bordered group, a row and an empty inspector, with reply actions visually attached to tools. A second screenshot shows a message profile without its available current launch chain and a confusing first-read layout.

## What Changes

- Use a default-folded grey activity summary and compact unboxed rows, preserving action icons and real input inspection.
- Remove empty inspectors and attach reply actions to the final text before trailing calls.
- Give profile reads stable loading, failure/retry and unrecorded states.
- Read a same-session current launch chain only on explicit expansion, separate from historical facts. Retain source/session verification, sanitized arguments and observation time.

## Surfaces

- **Terminal**: unchanged transcript/core contract; no new commands or status categories.
- **Menu bar**: unchanged; it opens the local session rather than embedding this conversation view.
- **Phone**: updated shared React Native conversation, profile loading and chain disclosure.
- **iPad**: the same components and behavior, preserving one-column large-text facts and bounded scrolling.
- **Web**: existing folded activity behavior and its own rendering remain; the reported mobile layout and profile dialog are the scope of this correction.

## Validation

Behavior regressions cover reader choices, input copy, missing arguments, delayed/error reads, target changes and runtime isolation. Native simulator captures use synthetic data. CI, author checks, independent review, physical devices and spoken VoiceOver remain separate evidence.
