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
Simulator targeting ARM64 independently of the runner host architecture, without signing, provisioning, uploading or installing. Runner-space
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


### Requirement: Parallel Android checks preserve one build identity

The selected Android CI SHALL build and lint the QA APK once, record its source and SHA256, and run the complete phone and tablet emulator suites in independent jobs using those identical bytes. Artifact names SHALL include run attempt. Missing artifacts, source mismatch, checksum failure, skipped or failed selected legs SHALL fail the aggregate. Each leg SHALL retain separate evidence.

#### Scenario: One emulator leg fails

- **WHEN** the phone passes but the tablet fails or is cancelled
- **THEN** the required aggregate fails, while phone evidence remains available

### Requirement: Early release preparation and bounded retrieval

Maintainers SHALL be able to stamp an explicit stable version without creating a tag and preflight version and bilingual notes for every selected mobile platform before opening the release PR. Android notes SHALL remain independent of Apple notes. Retrieval SHALL require an exact successful source/run/attempt/workflow and a nonexpired encrypted artifact with a published digest. GET retries SHALL be bounded, credentials SHALL NOT cross an origin boundary, and corrupted or partial downloads SHALL NOT be exposed as verified results.

#### Scenario: Android notes are missing

- **WHEN** an Android release is selected without that version's bilingual Android entry
- **THEN** the read-only preflight fails before a native build or tag publication

#### Scenario: The transport fails transiently

- **WHEN** an artifact GET fails before completion
- **THEN** retrieval may retry within its bound, and only complete digest-verified encrypted bytes become output

### Requirement: Verify actual native target coverage

Unsigned iOS CI SHALL retain the normal standard host and explicitly compile for ARM64 Simulator. It SHALL inspect the actual app and both embedded extension executables before success. Missing products or an unexpected architecture SHALL fail the gate. The complete native build SHALL pass on the actual candidate and main, without signing credentials or changes to the production delivery runner.

#### Scenario: An extension target drifts

- **WHEN** an embedded extension is missing or compiled for an unexpected architecture
- **THEN** the unsigned native gate fails even if xcodebuild itself returned success

### Requirement: Check effective public commit identity

Release preflight SHALL inspect the effective author and committer identities before platform preparation succeeds. It SHALL accept the project public or GitHub no-reply identity, redact rejected configured addresses, and never change shared Git configuration. A separate identity-only mode SHALL be available before ordinary public task commits. This check SHALL NOT claim removal of metadata already published.

#### Scenario: A work identity is inherited

- **WHEN** the effective author or committer uses an address outside the public identity policy
- **THEN** preparation fails without printing that address or changing configuration

### Requirement: Parallel desktop builds preserve publication ordering

The CLI release gate/build and Mac signing/notarization build MAY run concurrently
at the same official source. The Mac build SHALL have read-only repository access
and SHALL NOT upload release assets or update casks. A separate publisher SHALL
require both successful builds, consume only their run-scoped immutable Mac ZIP,
and verify its source, ref, run, original build attempt, size and trusted SHA256
before upload. It SHALL NOT rebuild or re-sign during transfer. The official signed
mirror SHALL wait for successful Mac publication as well as both builds. Historical
serial releases SHALL remain eligible for exact-source mirror validation.

#### Scenario: CLI fails while Mac succeeds

- **WHEN** the parallel Mac build passes but the CLI release gate fails
- **THEN** no Mac release asset, cask or signed mirror is published by that run

#### Scenario: Only publication is retried

- **WHEN** a failed publisher is rerun after its Mac build already succeeded
- **THEN** it verifies the original successful build artifact ID and build attempt
- **AND** a changed ZIP or artifact from another source/run is rejected

### Requirement: Mac host selection preserves universal product checks

Mac native checks and signed release builds MAY use different qualified standard
hosts. CI SHALL compile the app and Widget host/extension for arm64 and x86_64 and
run every native unit suite and design check on its selected host. A host with
native rendering failures SHALL NOT count as native UI acceptance, even if it
compiles a universal artifact. Release artifacts SHALL retain existing universal,
signing and notarization checks. iOS host selection and actual ARM64 product
verification SHALL remain independent.

#### Scenario: A Mac job changes hosted architecture

- **WHEN** a standard Intel host builds a signed release and native CI uses ARM
- **THEN** both product architectures are compiled and every native check runs on the qualified CI host
- **AND** official artifacts remain universal with the same publication gates

### Requirement: Reviewed test design and traceable case catalogue

Test additions, changes, moves and removals SHALL record observable expected behavior,
layer and planned sources in a reviewed design before implementation. The review SHALL
state its actual author/type and SHALL NOT fabricate independent or user approval.
PR CI SHALL verify that the sealed case design precedes test implementation commits.
Snapshot gates SHALL verify source hashes, case indexes and implementation selectors;
a normal final squash SHALL retain the reviewed record and snapshot checks. Legacy
inventory SHALL NOT be labelled retrospective approval or runtime passes.

#### Scenario: Implementation and design arrive together

- **WHEN** a PR introduces its reviewed design in the same commit as its first test edit
- **THEN** the test-design history gate fails even if snapshot hashes match

#### Scenario: An implementation drifts from its record

- **WHEN** a test source changes without updating its reviewed mapping and catalogue
- **THEN** validation fails instead of silently claiming coverage

### Requirement: Separate native oracles and manual tools

Affected iOS CI SHALL execute the production Foundation HTML and Markdown procedural
oracles with assertions enabled, separately from unsigned app compilation. Ordinary
RN/Appium regression collection SHALL exclude manual captures and diagnostic probes.
Explicit send diagnostics SHALL verify an owned loopback fake server and an exact,
unambiguous pane before device access, and SHALL NOT choose the first live agent.
Knowledge capture SHALL NOT perform knowledge promotion actions.

#### Scenario: Native document behavior fails

- **WHEN** the production parser fails a native document oracle
- **THEN** the iOS job fails even if the application would compile

#### Scenario: A diagnostic points at an ordinary service

- **WHEN** URL/token are supplied but the endpoint lacks the synthetic fixture identity or exact target
- **THEN** the send diagnostic refuses before accessing a device

### Requirement: Independent cross-client behavior assertions

Cross-client presentation regressions SHALL consume shared synthetic expectations
but exercise actual RN components and browser DOM independently. Fixtures SHALL cover
provider identities, source turn ordinals and record counts, HQ message provenance,
Markdown/code and scoped document actions. Passing shared pure helpers alone SHALL NOT
be reported as rendered parity or native device acceptance.

#### Scenario: One renderer uses the wrong count or label

- **WHEN** one renderer changes loaded-window counts, call labels or sender identity
- **THEN** its component/browser test fails against the shared expectation fixture
