# code-architecture (delta)

## ADDED Requirements

### Requirement: The knowledge base is one leaf package

The package `internal/knowledge` SHALL own everything about the ledger: format,
migration, vocabulary, provenance, audience, pool, promote/land/withdraw, lint,
neighbours, render, distribution and the API shapes. `internal/hq` SHALL keep only
supervision (sensors, playbook, verb dispatch shims). `knowledge` SHALL import nothing
above the leaves and `mine` SHALL NOT import `knowledge`; `check-design.sh` SHALL enforce
both.

#### Scenario: A knowledge verb is added in hq

- **WHEN** ledger logic appears in `internal/hq` outside the shim files
- **THEN** the design gate fails naming the file
