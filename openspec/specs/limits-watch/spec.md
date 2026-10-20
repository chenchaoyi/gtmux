# limits-watch Specification

## Purpose
Describe the agent-specific rate-limit windows exposed by gtmux and the metadata needed to
interpret them without changing existing limit reports.

## Requirements

### Requirement: Codex plan type is preserved as optional limits metadata

When Codex writes a `plan_type` with its rate-limit windows, gtmux SHALL trim surrounding
whitespace and expose the remaining value as additive metadata on its limits windows.
The value is passed through without checking it against a list of recognized plans. If
the field is absent, empty or all whitespace, gtmux SHALL leave the metadata empty and
preserve the existing window report.

#### Scenario: Codex rollout has a plan type

- **WHEN** a Codex rate-limit record contains `plan_type`
- **THEN** each window derived from that record includes the plan type

#### Scenario: Codex reports a new plan name

- **WHEN** a rate-limit record contains a non-empty plan name that gtmux has not seen before
- **THEN** each derived window preserves that name, with surrounding whitespace removed

#### Scenario: The plan type is unavailable

- **WHEN** the plan type is absent, empty or all whitespace
- **THEN** the windows are still reported with no plan-type metadata

### Requirement: Codex quota reads have a bounded tail allocation

For each considered rollout, the system SHALL open the file once, obtain its size
and modification time from that descriptor, and read at most the final 1 MiB plus
one preceding boundary byte from that size snapshot. Read buffers SHALL NOT grow
with the complete rollout size. The newest valid rate-limit record and existing
window/reset/plan semantics SHALL remain unchanged.

A first record cut by the tail boundary SHALL be ignored, including a suffix that
happens to be valid JSON. A complete record exactly at that boundary or a valid
final record without a trailing newline SHALL remain eligible. Appends beyond the
sampled size SHALL wait for the next poll. Short reads, including concurrent
truncation, and read errors SHALL discard that file's reading without retrying or
parsing partial bytes. This bounded read does not promise an atomic snapshot of
concurrent in-place rewrites.

#### Scenario: A rollout grows much larger than the tail budget

- **WHEN** a rollout contains a recent valid quota record after a large prefix
- **THEN** the quota is reported with bounded read bytes and allocations independent of prefix size

#### Scenario: The tail begins in the middle of a record

- **WHEN** the first tail fragment is not preceded by a newline
- **THEN** that fragment is skipped and later complete records remain eligible

#### Scenario: A rollout changes during the read

- **WHEN** bytes are appended beyond the descriptor's sampled size
- **THEN** they are deferred to the next poll without increasing this read's budget

#### Scenario: A read cannot supply its sampled tail

- **WHEN** truncation or a read failure prevents a complete bounded read
- **THEN** that file yields no reading and no partial record is accepted

### Requirement: Account metrics preserve capability and freshness

The system SHALL distinguish subscription windows, monetary wallets and local
token accounting. It SHALL expose only relevant installed or observed sources,
label recorded plan evidence, and retain historical usage after source removal.
Every metric SHALL retain its own observation scope/time; stale quota windows
SHALL NOT drive current warnings or the tightest-window summary. Native DeepSeek
Account balance collection SHALL use its own service, omit credentials and PII,
reject late results after account switches, and deduplicate verified account
observations. Unsupported API-key/custom-provider balance sources SHALL be stated,
not guessed or rendered as zero.

#### Scenario: An account changes during a balance request

- **WHEN** the native account changes before a balance request finishes
- **THEN** the old result is discarded and cannot populate the new account
