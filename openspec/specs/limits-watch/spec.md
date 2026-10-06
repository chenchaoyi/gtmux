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
