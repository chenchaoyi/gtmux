# Completed checks and actionable findings

## Why
A completed report was labelled as items still waiting to be checked. The count
actually combines recommendations and blocking findings. Plain doctor only prints
counts at the end, so users have to search a long report for flagged rows; the fix
pass has a separate remaining-items list but treats recommendations as failures.

## What changes
- Keep the four existing result classes; make completion distinct from severity.
- End plain doctor with a complete, severity-ordered findings list carrying group,
  current result and the existing guidance. Reuse it after a fix pass.
- Put a compact findings summary at the beginning of the Mac report, with top and
  bottom navigation to it and links to the original check and its details.
- Explain the four classes in the Mac window and docs; use consistent result copy.

## Five-surface impact
- CLI: final findings list and clearer result wording, unchanged checks/exit codes.
- macOS: summary and navigation in the existing read-only report window.
- iPhone and iPad: no diagnostics controls or report changes.
- Web: no diagnostics controls or report changes.

## Boundaries
No automatic repair, HQ rotation or real configuration changes. Read-only rechecks
may return the same findings until their underlying condition changes. JSON schema
and counts remain compatible; informational rows are never action items.
