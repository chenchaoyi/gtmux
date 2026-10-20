# ci-checks Specification

## Purpose
Choose PR checks conservatively, expose the chosen scope, and fail incomplete
validation without involving release credentials or deployment approval.

## Requirements

### Requirement: Conservative check selection

PR and main-push CI SHALL always run source conformance, strict specs, bilingual
pairing, generated documentation examples, workflow validation and CI guard tests.
Only recognized documentation-only paths MAY omit platform suites. Known module
changes SHALL run complete corresponding suites and explicit downstream consumers;
mixed changes SHALL use the union. Shared core/API, dependencies, build/CI changes,
unknown impact, changed symlinks or unavailable diff information SHALL run all suites.
Rename classification SHALL include the old and new paths; deleted paths SHALL be
classified too. Manual CI SHALL run all suites. CI SHALL explain its selection.

#### Scenario: Prose changes

- **WHEN** a PR changes only recognized Markdown documents
- **THEN** policy and documentation gates run while native/platform suites can skip

#### Scenario: Uncertain or mixed impact

- **WHEN** diff resolution fails, a new unknown component is added, or a core file is renamed into docs
- **THEN** all platform suites are selected

#### Scenario: Module changes

- **WHEN** a PR changes menu-bar Swift code and Web styles
- **THEN** Swift, complete Go and Web suites run, while unrelated phone/Worker suites can skip

### Requirement: Stable result aggregation

The always-running aggregate SHALL require successful scope, policy and every
selected suite. Missing, failed, cancelled or skipped required suites SHALL fail
the aggregate. An unexpected failure in an unselected suite SHALL also fail.
The classifier tests SHALL discover workflow jobs so an added suite cannot silently
escape aggregation. PR CI SHALL use read-only permissions and no delivery secrets.
Manual check cancellation SHALL NOT cancel the main-push check used by test delivery.

#### Scenario: A selected job skips

- **WHEN** native compilation is selected but the native job is skipped
- **THEN** the aggregate fails rather than reporting the PR as checked

#### Scenario: Manual full check overlaps a main check

- **WHEN** a manual full check starts while the main-push check is running
- **THEN** it runs in a separate cancellation group and the main check continues

### Requirement: Rendered and native build checks

Affected Web CI SHALL exercise real embedded assets against a synthetic loopback API
in Chromium, covering enrollment/retry, private-content revocation, live Chat refresh,
sidebar/restore controls and read-only narrow-screen layouts. Failure evidence SHALL
be retained. Affected mobile CI SHALL compile the app and its extensions for iOS
Simulator using the runner's architecture, without signing, provisioning, uploading or installing. Runner-space
preflight SHALL fail before a native build with insufficient space.

#### Scenario: A narrow toolbar overflows

- **WHEN** a CSS change makes the plain-shell toolbar wider than a narrow viewport
- **THEN** the rendered browser suite fails and retains a screenshot/trace

### Requirement: Incomplete scanner output is a failure

The vulnerability gate SHALL fail scanner execution failure or missing/invalid JSON
protocol output. It SHALL continue to fail reachable advisories with an upstream fix
and warn on reachable advisories without a fix.

#### Scenario: Scanner startup fails

- **WHEN** the scanner exits nonzero without JSON findings
- **THEN** CI fails and does not claim no reachable vulnerability

### Requirement: Distinguish validator startup failure from invalid specifications

The design gate SHALL require a complete structured verdict from the pinned OpenSpec validator for the current checkout with a nonempty inventory and consistent totals. Invalid specification results SHALL fail as spec errors. Missing tools, invocation/cache failures, timeouts, malformed output, contradictory success and checkout mismatch SHALL fail as tool/setup errors, never success or an asserted malformed-spec verdict without evidence.

#### Scenario: npx cannot start the validator

- **WHEN** the process cannot execute or does not produce a complete current-checkout report
- **THEN** the gate fails with a tool/setup diagnosis

#### Scenario: The validator reports an invalid item

- **WHEN** a complete report contains a failed specification even with process exit zero
- **THEN** the gate fails with the actual invalid item

### Requirement: Keep operator privacy policy outside public checker source

Exact personal/operator terms SHALL be read from private CI configuration or a
private local JSON policy, never embedded in the public checker. The check SHALL
inspect tracked paths and text, including checker code and fixtures, without
printing policy values or matched source content. Configured but missing, malformed
or unreadable policy and source-read failures SHALL fail the check. The wrapper
SHALL remove policy variables before starting unrelated build/spec helpers.

Repository-owned PR, main-push and manual policy jobs SHALL require the private
policy. Fork jobs SHALL NOT receive it and SHALL explicitly disclose skipped
exact-term coverage; maintainers SHALL apply the private check before merging an
outside contribution. The check SHALL NOT claim image-content, general-secret or
historical-commit coverage.

#### Scenario: A checker contains a private term

- **WHEN** a tracked checker or fixture reintroduces a configured private identifier
- **THEN** the check fails without exempting its own source or echoing the identifier

#### Scenario: Private policy is unavailable

- **WHEN** a repository-owned policy job has no usable private policy
- **THEN** it fails instead of claiming privacy coverage
- **AND** a fork job without that policy explicitly reports its coverage gap
