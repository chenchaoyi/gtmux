# Distinguish knowledge findings from navigation hints

## Why

The lint and self-check present 265 valid standalone entries as "orphans" and 27
textually similar pairs as if both counts were defects. A link is optional in the
knowledge ledger; similarity alone cannot establish duplicate meaning.

## What changes

- Keep existing JSON check names for consumers, and add a `severity` field.
- Mark unlinked entries `info`, similar pairs `review`, and make their text and
  self-check summary state the distinction.
- Preserve the comparison signal so HQ can review each pair against facts,
  provenance, and scope before changing the ledger.
- Include the alternate-language body when checking wiki links and graph edges;
  otherwise a broken English link is invisible beside a Chinese primary body.

## Surfaces

- **Terminal / 终端:** lint output and HQ self-check summary name advisory findings.
- **Phone / 手机, iPad / 平板, Menubar / 菜单栏, Web / 网页:** no interface change.
