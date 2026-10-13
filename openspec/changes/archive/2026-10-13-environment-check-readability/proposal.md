# Readable, correctly grouped environment checks

## Why
The native report uses row indices as IDs in every nested List section. Repeated
identities allow rows from the installation group to appear under unrelated
headings. Technical labels and tiny default captions also fail to explain what
is checked, while long values pull reading across the entire window.

## What changes
- Give every displayed row a section-qualified identity and render explicit groups.
- Add bilingual human-readable check labels and purpose descriptions to doctor JSON.
- Present check name, current result, purpose and expandable technical details separately.
- Use existing Theme fonts, localized check time and constrained reading width.

## Five-surface impact
- macOS: updated report window; repair and retry actions unchanged.
- CLI: additive JSON purpose field and clearer JSON labels; human terminal report unchanged.
- iPhone and iPad: no environment report or repair UI changes.
- Web: no environment report or repair UI changes.

## Scope
Read-only diagnostics presentation. No automatic repair, installation, notification
or service changes. Older reports without descriptions remain readable.
