# hq-wake-protocol (delta)

## MODIFIED Requirements

### Requirement: Signal visual language for injected lines

Every injected wake line SHALL open with the sigil `»` followed by
`gtmux·<class>` and columnar fields separated by `│`, so signal traffic is
visually distinct from conversational text in the HQ pane at a glance. The sigil
and separators SHALL be chosen for encoding robustness (survive a missing/POSIX
locale) and the format SHALL be pinned by a fixture test. Agent-authored content
in the line (titles, goals, reply tails) SHALL remain quoted-and-labelled as
DATA per the existing nudge-payload convention.

Every wake line SHALL additionally carry, at a fixed position in the grammar, an
ATTENTION-GRADE glyph from a three-value scale — decision (needs the commander:
irreversible, money/safety consequence, or an explicit ask) · attention (a line is
blocked or a fleet change worth knowing) · ledger (bookkeeping, zero interrupt value) —
projected DETERMINISTICALLY from the line's class (and severity where the class spans
grades). The projection SHALL live in one place beside the class table, every declared
class SHALL have a grade (conformance-tested), and the glyphs SHALL meet the same
encoding-robustness bar as the sigil. The grade adds structure a terminal reader can
scan the way color would be scanned — the composer and the agent TUI cannot carry ANSI
color, so the glyph IS the in-pane color.

#### Scenario: A wake line is visually distinct

- **WHEN** any wake-class or tick line is injected into the HQ pane
- **THEN** it opens with `» gtmux·<class>` and uses `│`-separated fields

#### Scenario: The format survives a hostile locale

- **WHEN** the wake line is rendered under a POSIX/C locale environment
- **THEN** the sigil and separators still render without mojibake (pinned by test
  fixtures)

#### Scenario: Every line announces its grade

- **WHEN** a `waiting·kind` wake (an explicit ask) and a `tick` wake are injected
- **THEN** the former carries the decision-grade glyph and the latter the ledger-grade
  glyph, each at the same fixed position, pinned by the fixture tests

#### Scenario: No class ships ungraded

- **WHEN** a new wake class is declared without an entry in the grade projection
- **THEN** the conformance test fails the build
