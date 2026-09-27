# limits-watch Specification

## Purpose
Describe the agent-specific rate-limit windows exposed by gtmux and the metadata needed to
interpret them without changing existing limit reports.

## Requirements

### Requirement: Codex plan type is preserved as optional limits metadata

When Codex writes a `plan_type` with its rate-limit windows, gtmux SHALL expose that value as
additive metadata on its limits windows. If the field is absent or unknown, gtmux SHALL leave
the metadata empty and preserve the existing window report.

#### Scenario: Codex rollout has a plan type

- **WHEN** a Codex rate-limit record contains `plan_type`
- **THEN** each window derived from that record includes the plan type
