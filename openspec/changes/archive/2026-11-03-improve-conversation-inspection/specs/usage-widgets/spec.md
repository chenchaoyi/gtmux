## ADDED Requirements

### Requirement: Compact shared medium widget layout

The shared Apple medium usage view SHALL avoid duplicate stacked identity headers, keep the primary value and window label together, and retain visible source and reading time within its fixed frame. Single-provider zero, stale, expired and long-label states and multi-provider output/quota states SHALL preserve existing units and freshness semantics in light and dark appearance.

#### Scenario: A provider reports zero weekly usage

- **WHEN** the medium widget shows one provider with a long server name
- **THEN** zero remains an actual zero and the updated reading footer remains visible without a second identity header
