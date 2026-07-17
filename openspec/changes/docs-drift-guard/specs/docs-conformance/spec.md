# docs-conformance — delta

## ADDED Requirements

### Requirement: A documented example of a code-produced format is rendered by that code

A documented example of a code-generated format SHALL be marked as a rendered region
naming a registry id, and CI SHALL fail when the region's content differs from what that
registry entry's builder actually produces — printing both. The registry SHALL live in Go beside the builders, so
it cannot compile against a builder whose shape changed. A registered id with no
corresponding region SHALL also fail (a dead entry is drift too). Regions are OPT-IN: an
unmarked example is deliberately unchecked. CI SHALL only report; a separate command
rewrites the regions from the code.

#### Scenario: A builder changes and the doc does not

- **WHEN** the wake-line builder's format changes and a documented example still shows the
  old rendering
- **THEN** the docs check fails, naming the region and showing both the documented and the
  produced line

#### Scenario: A doc example is regenerated rather than hand-copied

- **WHEN** a contributor runs the docs-fix command after a builder change
- **THEN** the marked regions are rewritten from the code and CI passes, with no line
  hand-transcribed

#### Scenario: A registered example loses its region

- **WHEN** a marked region is deleted from the doc but its registry entry remains
- **THEN** the check fails rather than silently checking nothing

### Requirement: The wake vocabulary is complete wherever it is taught

Every wake class the code can emit SHALL appear both in the user-facing class table
(`docs/cli.md`) and in the seeded HQ playbook — the two surfaces that teach the vocabulary
to the human and to HQ respectively. CI SHALL fail on any class missing from either, so a
newly added class cannot reach a reader (or a supervisor) as an unexplained glyph. A class
deliberately not surfaced SHALL be exempted through an explicit allowlist, mirroring the
existing command-registry check's `HIDDEN`.

#### Scenario: A new wake class is added and documented nowhere

- **WHEN** a wake class constant is added without a row in the class table or a mention in
  the playbook
- **THEN** the docs check fails, naming the class and the surface it is missing from

#### Scenario: An internal class is exempt on purpose

- **WHEN** a class is listed in the allowlist
- **THEN** the check passes without requiring it to be documented

### Requirement: Retired vocabulary cannot return

A token retired by a shipped change SHALL be denied in the documentation tree — a
superseded wake format, a deleted package path, a description a change explicitly killed —
and each denylist entry SHALL name the change that retired it, so the list reads as an
audit trail and an entry can be retired in turn.

#### Scenario: A superseded format reappears

- **WHEN** documentation reintroduces a retired token (e.g. the pre-hq-perception-v2
  `[gtmux] ` wake prefix)
- **THEN** the docs check fails, naming the change that retired it

### Requirement: The gate states its own boundary

The conformance gate SHALL NOT be presented as verifying that documentation is TRUE. Its
scope is claims with a machine-readable source: rendered formats, enumeration membership,
and retired tokens. Whether a doc's prose correctly describes behavior — for example that
a completion wake fires for any session rather than only a dispatched task — SHALL remain
an explicit review-gate item, stated as such where the rules are recorded, so a green gate
is never read as a reviewed doc.

#### Scenario: Prose remains a review-gate item

- **WHEN** documentation misdescribes behavior in a sentence with no machine-readable
  source
- **THEN** the gate does not claim to catch it, and the repo's rules record that judgment
  as a reviewer's responsibility
