# Participant profiles — visual acceptance

Compared the approved three-state reference with the native Release app built from
this worktree, using synthetic conversations and credentials. No live agent session
or account was used. This is author verification, not independent review.

## Evidence and findings

- Reference: approved conversation / agent / human board (1448 × 1086).
- Native iPhone: 402 × 874 pt, 3× captures; chat, per-turn collapsed preview,
  agent facts, expanded execution chain and human quote.
- Native iPad: 834 × 1210 pt, 2× captures; same flow in the split conversation.
- Combined comparison: operator-local `ops/profile-research/visual-comparison.png`.
  App-owned content was compared separately from the existing native shell. The
  reference is an illustrative board; its device chrome is not an app requirement.
- Typography: existing system font, readable hierarchy, selectable values/commands.
  Different source data wraps naturally; no fixed six-field template.
- Spacing: centered charcoal panel, bounded height, attached reply footer and 44pt
  targets. Per-turn preview retains two lines and its own expand control.
- Colors: dark neutral surface, muted labels, clear primary values; existing cyan
  human avatar and official bundled agent imagery retained.
- Assets: existing repository avatars/icons used; no replacement brand artwork.
- Copy: approved “碳基决策者 · 自封” and Dune quote retained. Extra humorous paragraph
  absent. Copy/share remain icon-only with accessibility names.
- Intentional differences: native Close text uses the existing text control; source
  details are a separate disclosure, facts reflect native evidence, and launch-chain
  observations carry a time. Privacy omits unstructured arguments from the mock.

No actionable P0/P1/P2 visual mismatch in the normal-size phone/iPad captures.
An initial UI fixture used a nonexistent reply field; correcting it to the real
`response` contract restored reply content. A tablet trial inherited an old extreme
font setting and clicked an off-screen avatar; the corrected scoped flow passes.
These were fixture/driver failures, not silently counted as product passes.

## Acceptance boundaries

Phone/iPad synthetic native interaction runs passed for profiles and local folding.
Physical devices, real model turns, independent review and VoiceOver speech are not
verified. Accessibility metadata is present; speech/focus behavior needs device
acceptance. Extreme Dynamic Type is checked separately below.

## Extended text iteration

P2 found: two columns fragmented a long model identifier at
`accessibility-extra-large`. Fixed by switching the facts grid to one column when
fontScale exceeds 1.3. A fresh native Release build and real-touch run passed;
`large-fixed-agent-top.png` shows the identifier intact and
`large-fixed-chain.png` shows scrolling with Close still reachable. The combined
`visual-responsive-comparison.png` includes the source, normal state, failed state
and corrected capture. The initial driver toVisible scroll hit its max scroll
count; real pointer gestures verified the actual nested scroll instead.

No remaining actionable P0/P1/P2 visual finding. Layout comparison does not prove
VoiceOver speech or physical-device behavior.

final result: passed
