# Compact activity and readable profile loading — 2026-10-20

This follow-up compares the two user-supplied phone screenshots with synthetic
normal-size iPhone simulator captures. This is author verification, not independent
review. The earlier participant-profile acceptance below is historical evidence,
not a substitute for this iteration’s coverage.

## Findings and correction

- Removed the bordered activity card and the repeated Wait inspector. Both live
  and historical groups start as a grey disclosure; a parameter-free Wait is one
  static icon row. Real command/path input retains inspection, bounded vertical
  scrolling, horizontal scrolling and literal copy.
- Moved the unboxed reply footer directly after the final text, before trailing
  calls. Copy/Share remain icon-only and the reply fold stays with the text.
- Kept the profile’s body size stable while a labelled static placeholder loads.
  Unrecorded, failed/Retry, and ready states are distinct. Facts never flash from
  another target. The compact identity leaves room for facts and chain access.
- Added explicit same-session current-chain disclosure beside historical facts.
  The synthetic `tmux → zsh → node → codex` path is visible with observation time
  and sanitized-command limitations; historical model/tier/effort remain unchanged.
- P2 found during scrolling: body content passed under Close. Reserving a separate
  close-control band fixes the overlap; the final chain capture was rechecked.

## Evidence and boundary

Operator-local evidence lives under `ops/secondary-evidence/`, with bundle and
component hashes in `ops/native-preview-manifest.json`. Source screenshots were
viewed alongside the revised captures. Accepted tool captures cover the static
Wait, command input before/after a real horizontal gesture, loading, ready facts
and all four chain nodes. Captures are the UI tool’s optimized 368 × 800 images,
not full-resolution device screenshots. The earlier Wait capture uses the same
unchanged ToolSteps component; profile captures use the captured JS bundle identified in that manifest.

To avoid another multi-gigabyte native build, this preview installs an existing
Release simulator native shell (1.0.137) with this worktree’s bundled JS. Native
code did not change. This is not evidence of a new native compilation, a signed
phone package, or an installed production build. CI is reported separately.
The fixture used synthetic credentials/logs and executed no real pane commands.
Profile request observations confirm a separate historical read and same-session
current read, without automatic polling.

Some UI taps initially did not change the screen, and the working-state timer
prevented the driver’s generic settle predicate. Fresh snapshots and actual
screenshots verified accepted interactions; the ineffective taps were not passes.
Copy and failure/Retry contracts are covered by component tests, not native
pasteboard or fault-injection acceptance in this iteration.

No remaining actionable P0/P1/P2 visual finding in the inspected normal-size
phone flow. The captures precede a final error-only correction: an unavailable
server read with no chain now shows failure/Retry rather than unrecorded. That
branch is component-tested; the inspected normal flows did not change.
This iteration has no iPad/large-type, Android, physical-device,
independent-review or VoiceOver speech/focus acceptance.

final result: passed within the stated normal-size phone scope

---

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
