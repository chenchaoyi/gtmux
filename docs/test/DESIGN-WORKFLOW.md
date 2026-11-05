# Test design workflow

[中文](DESIGN-WORKFLOW.zh.md)

Design and review precede test implementation. This initial record was self-reviewed by the implementation owner before changing tests; it is not independent approval.

1. Record stable case IDs, layer, surfaces, setup, actions, observable expectations, negative/boundary cases, planned files and evidence limits in `docs/test/designs/`. Review the design and commit its sealed record before changing test code. A review by an agent must say so; do not fabricate user or peer approval.
2. Implement the reviewed cases. Map each test change to its design IDs and concrete implementation selectors. A scope or expected-result change requires another design review before implementation.
3. Run affected tests, negative guard examples and the complete core gate. Record actual results, skips and device limitations separately from design status.
4. CI checks the catalogue, seals, mappings and PR commit order. It does not judge whether prose is true or whether a human genuinely reviewed it. The reviewer must assess assertion quality and design/code agreement.
5. Legacy catalogue entries describe existing source, not retrospective reviewed designs or execution passes. No mass backfill of fictional reviews.

## Commands and record format

Use `designs/test-system-20261105.json` as a concrete record example. Each case has
`id`, `layer`, `description`, `setup`, `action`, `expected`, and `planned_files`;
add the affected products to the description/setup. The review states reviewer,
date, decision, and rationale/limits. Mark draft records `draft` during preparation;
CI intentionally rejects them until the review is recorded and status is `reviewed`.

```sh
# After reviewing the design; no test implementation in this commit yet:
python3 scripts/test_design.py --seal-plan docs/test/designs/<change>.json
git add docs/test/designs/<change>.json
git commit -m "test design: review <change>"
# Then implement tests and update implementations.json with case IDs/selectors.
python3 scripts/test_design.py --refresh
make test-design
# Before pushing, also check the PR's exact base/head ordering:
python3 scripts/test_design.py --pr-history --base <base-sha> --head <head-sha>
```

`implementations.json` maps source paths to reviewed `cases` and literal executable
`selectors` (use full test names). Removed paths use `removed:true` and a reason;
new and old paths are both recorded for moves. Refresh refuses unreviewed/unmapped
source changes. It updates source hashes and mapped selectors; new selector rows
are explicitly typed `reviewed-selector`, not claimed framework test discoveries.
Review the CSV diff, preserve legacy declaration records, and remove retired entries
with their rationale. Dynamic templates remain templates; no fabricated runtime count.

Seals are not signatures. PR CI reads prior commits and rejects a design introduced
with or after the first test edit. Keep those commits until the PR is evaluated.
After normal squash, main/release verify the resulting content; the successful
PR check is the evidence for development order. Do not treat main's snapshot as
independent proof of chronology. If a design changes, commit its new reviewed seal
before implementing that change, and keep earlier case expectations auditable.

An existing test edit cannot reuse an entirely unchanged historical review seal: record and commit a fresh review first. Stable case IDs may remain; the review rationale/expectations and mapping must reflect this change.
