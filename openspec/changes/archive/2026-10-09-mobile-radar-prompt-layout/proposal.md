## Why

A phone screenshot shows a long Codex prompt covering terminal output, and the radar can scroll into a large blank tail. Its footer count also reads like missing sessions when a section is folded.

## What Changes

Keep the Codex prompt entrance bounded; show the complete instruction in an independent reading sheet. Replace the radar content minimum-height extension with a bounded native bottom inset and explain folded rows explicitly. Preserve the guarded prompt match and session discovery.

## Surfaces

- terminal: no CLI or Codex configuration changes.
- menubar: unchanged.
- phone: bounded prompt entrance, separate reader, bounded radar clearance and folded-count wording.
- iPad: same prompt reader; sidebar has no floating-disc clearance.
- Web: unchanged.

## Impact

Mobile components, regression tests, mobile-app spec and bilingual design documentation. No server API, identity or state changes.
