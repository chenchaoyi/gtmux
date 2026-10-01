# A readable board outline with clear disclosures

## Why
The board mixes tiny font-dependent triangles with isolated paragraph carets. A long location competes with a bare pane ID, while the task is hidden. Expanded table fields form dense side-by-side text. Empty decision headings still appear as outline entries, and lifted decisions repeat in the outline.

## Design
Use one 16-point stroke chevron in section, entry and table headers, at the trailing edge of a whole-row button at least 44 points tall. Short text buttons say Show full text / Show less instead of bare carets. Preserve author order, source facts and opt-in folding. Pane previews prioritize an explicitly labelled task, retaining their original pane ID and location as secondary lines. Expanded fields stack labels above selectable values; long values can reveal full text in place. Empty entries are not controls and a lifted decision section appears only once. No new animation.

## Surfaces
- phone: shared board reader, clear disclosures and field hierarchy.
- iPad: same reader in its form sheet; controls and text wrap within the available canvas.
- terminal: unchanged Markdown and board storage; this is a reader presentation change.
- menubar: no board-renderer change in this batch; its native disclosure controls are separate.
- Web: unchanged board storage and Web renderer; mobile-only report and opt-in Markdown folding.

## Verification plan
Synthetic board/component regressions cover empty and duplicate decisions, whole-row actions, full-text reveal, task/location previews, intact fields and expansion across polling. TypeScript/lint/Jest and repository design/OpenSpec gates. Avoid a new native build when an existing debug simulator app can validate JS changes. Physical unlocked-device reading remains a separate acceptance step.
