# Test design workflow

[中文](DESIGN-WORKFLOW.zh.md)

Design and review precede test implementation. This initial record was self-reviewed by the implementation owner before changing tests; it is not independent approval.

1. Record stable case IDs, layer, surfaces, setup, actions, observable expectations, negative/boundary cases, planned files and evidence limits in `docs/test/designs/`. Review the design and commit its sealed record before changing test code. A review by an agent must say so; do not fabricate user or peer approval.
2. Implement the reviewed cases. Map each test change to its design IDs and concrete implementation selectors. A scope or expected-result change requires another design review before implementation.
3. Run affected tests, negative guard examples and the complete core gate. Record actual results, skips and device limitations separately from design status.
4. CI checks the catalogue, seals, mappings and PR commit order. It does not judge whether prose is true or whether a human genuinely reviewed it. The reviewer must assess assertion quality and design/code agreement.
5. Legacy catalogue entries describe existing source, not retrospective reviewed designs or execution passes. No mass backfill of fictional reviews.
