# Offline demonstration coverage

The pairing screen’s **See a demo** opens the production phone or iPad shell over
`ui/demoClient.ts`. There is no deployed public demo server. The App Store review path
uses this offline tour. `e2e/fake-serve` is a local test server; `seedDemo` reads the same
fixtures and decision script for screenshots and HTTP client checks.

## Coverage

| Area | What the tour demonstrates |
|---|---|
| Radar and navigation | Shared compact/regular shells, fleet counts, filters, HQ entrance and all panes, including plain terminal neighbours |
| Conversation and terminal | Production readers, Markdown, reply controls, synthetic input and approval → working → completion; refusal never runs tests |
| Codex | Intermediate text and tool segments, plus a scoped desktop work conversation in the production read-only reader; no desktop input or follow administration |
| HQ | Assessment, board, conversation, dispatches and activity; the current permission decision is reflected across views |
| Usage | Plan windows, same-fleet session identities, derived agent totals, seven daily totals and activity history |
| Knowledge | Topic counts and newest-first entries, specific bodies, promotion examples and a bilingual entry |
| Isolation | Sample labels, synthetic history, no server credential, no real input, cancel pending scripted transitions on exit |

The sample depicts one macOS host. It does not validate Linux runtime behavior. The new
macOS HQ workspace is a native-only surface; the demo uses the actual phone/iPad HQ view.

## Intentional exclusions

Real session creation, pairing/sharing administration, route changes, access history and
push registration need a paired host or real device identities. Those entrances remain
unavailable in the tour. Demo knowledge mutations return an explicit refusal. Route
mutation cannot report a fake success. These exclusions are not proof that real server
administration, notifications or network recovery work.

## Keeping it aligned

New client methods must be implemented or explicitly classified in `NotInDemo`. A method
used by a shared reader or composer cannot be excluded: production Detail uses
`sendResult`, so the tour implements its receipt and retry path too. Derive panes and
usage identities from the same fleet. Keep the HTTP seed on this script rather than
implementing a second approval simulator.

`demoParity.test.ts` checks distinct outcomes, stable timestamps, reset/cancellation,
receipt retries, coherent counters and scoped read-only data. `fake-serve/demo.test.ts`
checks the real HTTP client against the shared seed. `DesktopConversationView.test.tsx`
checks the production reader with the demo client and no input field. These checks do
not establish physical phone/iPad layout, VoiceOver, APNs or Live Activity acceptance.
