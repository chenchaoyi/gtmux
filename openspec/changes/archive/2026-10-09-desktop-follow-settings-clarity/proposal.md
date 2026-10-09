# Simplify desktop conversation settings

## Why
The shipped form repeats saved state, mode choices, explanatory paragraphs, a warning and scope text. The hierarchy makes a three-boolean decision look like a complex workflow. Mac also exposes default-valued controls after an initial load fails.

## What changes
- One **HQ follow** switch with a short reading/reporting explanation replaces the mode radios and saved-state chip.
- When enabled, a clearly grouped **Notifications and knowledge** section contains two independent, initially-off choices. Each has one sentence describing its effect.
- Scope sits once under conversation identity. Continuing in ChatGPT is one quiet footer hint; stopping shows the existing-record retention note only when relevant.
- A fixed footer keeps Cancel/Save reachable and reports unsaved/saved state. Loading and failed initial reads do not expose editable defaults.
- Keep the revision-checked save, independent permissions, revocation and future-only knowledge boundaries.

## Non-goals
No policy/backend changes, new agent support, publishing, installation or notification delivery changes.

## Surfaces
- Terminal: existing CLI choices and receipts remain; no UI form.
- Menu bar: compact utility form, master switch and two grouped capabilities, native keyboard actions.
- Phone: shared bottom sheet with the same hierarchy, readable rows and accessible switches.
- iPad: the same bounded component, scrollable content and fixed footer; no separate implementation.
- Web: status-only diagnostic list remains; there is no settings writer to redesign.

## Verification
Regression tests cover permission independence, stop/re-enable, initial read failure, save/conflict receipts and duplicate-write protection. Native rendering checks cover both languages/appearances and form states. Phone/iPad physical layout and VoiceOver acceptance remain distinct from automated checks.
