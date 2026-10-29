# Make the workspace read like one product

## Why
Current screenshots expose an oversized turn directory in focused Chat, four
unrelated controls styled as equal tabs, heavy tool-call cards, weak HQ identity,
crowded select arrows and an empty workspace with no useful first action.

## What Changes
- Keep Chat and Terminal as presentation modes. Open Trace and Diff from session
  tools without replacing or resetting the underlying conversation or composer.
- Use the same conversation renderer in panels and focus, with a bounded reading
  column and compact, unboxed activity disclosures.
- Pin the actual supervisor role in the session list, improve form spacing and
  typography, and distinguish the local workspace from remote access on macOS.
- Give the empty workspace a clear session action and a short, independently
  selected Dune opening excerpt, with verified source and attribution.
- Reuse the existing authenticated conversation-document contract for attachments;
  never open an arbitrary local path or transmit file content to another service.

## Surfaces
- terminal: no command or terminal rendering changes.
- menubar: consistent footer action typography and a distinct local-window icon.
- phone: browser tools use the whole reading area; native Trace follows separately.
- iPad: browser tools adapt to actual width; native Trace uses the same tool model.
- Web: conversation, tools, HQ, forms and empty workspace changes above.
