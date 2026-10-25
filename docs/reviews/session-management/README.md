# Matter conversations: review evidence

The native **Conversations & records** sheet now starts with identifiable work and background records for the named matter. Reading and adding are primary; record ranges, individual moves and whole-matter organization appear only when requested.

See the [full illustrated review (Chinese)](README.zh.md) for the supplied before image, verified object semantics, numbered flow, accessibility risks, retained/hidden concepts and evidence boundaries.

1. **Identify included conversations — checked.** [After list](02-after-list-zh.png), [English](layout-list-en-660-light.png). Exact terminal availability, process status and collection range are independent. Status-since and saved-record update clocks are labelled honestly.
2. **Read original messages — exercised.** The native child-sheet screenshot is blocked by the capture environment; narrow geometry and close controls are checked, while its appearance remains visually unverified. Reading does not dispatch an instruction.
3. **Add existing work — exercised.** [Choose source](03-choose-conversation-zh.png), [review range](04-add-range-zh.png). Current conversations default to now; saved-only sources use a bounded historical reference.
4. **Adjust one conversation — exercised.** [Actions](06-adjust-actions-zh.png), [range](05-adjust-range-zh.png), [move](08-move-zh.png). Confirmations retain exact scope and revision; cancelled actions do not write.
5. **Split or merge the matter — exercised.** [Organization](layout-organize-en-350-dark.png). Splits move chosen associations; merge collects all conversations and HQ discussion, closes the source and retains originals. A prior individual move cannot preselect a whole-matter destination.

Screenshots are real SwiftUI test-window content rendered from synthetic fixtures, not installed-app or device acceptance. Layout tests cover English/Chinese, light/dark and 350/660 pt. [Narrow Chinese](layout-list-zh-350-light.png), [narrow dark English](layout-list-en-350-dark.png). The supplied before image is a user crop with different pixel bounds; this is not a pixel-fidelity comparison.

Native in-process AX actions exercise actual controls against an isolated `HQWorkspaceServing` fixture. Destination and new-matter text are fixture-seeded; review, confirm and cancel buttons are pressed. Tests inspect submitted scoped metadata and retained failed drafts. Core race regressions independently cover durable association/provenance behavior. No live HQ input or associations are touched. No phone/iPad device, installed app, system VoiceOver speech or complete manual keyboard acceptance is claimed. No installation, release, merge, version or store-material change is included.

Reusable principles: identify the work before exposing its metadata; separate individual and whole-matter scope in both navigation and confirmation; distinguish process status, record collection and goal completion; label clocks by what they actually measure. These principles are guidance, not additional acceptance claims.

Validation: `make check`, the complete design gate, 567 Swift tests and the release build passed. Native child-sheet bitmap export returned blank images, and capturing their own window IDs failed. Those invalid images are excluded. Confirmation controls were exercised, but native confirmation-sheet appearance remains visually unverified.
