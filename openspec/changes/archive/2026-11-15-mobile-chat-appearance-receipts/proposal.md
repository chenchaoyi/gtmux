# App conversation appearance and quiet send feedback

## Why
App Appearance already offers System, Light and Dark, but Chat forces a dark canvas. A confirmed send also leaves a large Received line between messages. Both contradict the surrounding interface and distract from the conversation.

## What changes
Use the existing persisted App Appearance preference for Chat and its shared reading controls. No second chat-specific preference. System changes update the open conversation in place. Keep terminal screen colors owned by the host. Show sending and ingress acknowledgement as small accessible marks under the user avatar within its existing 44pt footprint; keep queued work explicit. When the transcript confirms the message, remove the mark without remounting or resizing the row or bubble. Never show a receipt on an already recorded message, or turn elapsed time into delivery/execution proof. Preserve identity, attachments, failure recovery and send idempotency.

## Surfaces
- Terminal (incl. attach): no behavior change; host terminal palette stays independent, including embedded Live output.
- Menubar: no Chat reader or App Appearance preference; no change.
- Phone: Chat/HQ/external conversations and nested readers follow App Appearance; compact send feedback.
- iPad: same shared components and preference, retaining current regular layout and reading width.
- Web: existing browser theme choice remains independent; no browser implementation change in this App request.

## Validation boundaries
Use production React Native renderers and isolated deferred/persisted preference fixtures across agent types. Verify dark/light/system changes, readable markdown/tool details/avatars/controls, stable send handoff and explicit queue state. No real session sends, screen/TCC, device installation or service restart. Synthetic rendering does not establish physical-device or spoken screen-reader acceptance.
