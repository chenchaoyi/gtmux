## ADDED Requirements

### Requirement: Bound git diff output at the subprocess boundary

Pane diff capture SHALL read at most a 400 KiB aggregate prefix of branch, tracked diff and untracked-name output before adding its truncation marker. It SHALL stop an oversized producer and enforce an eight-second aggregate command deadline instead of materializing full subprocess output before trimming it. A non-repository SHALL retain the empty diff behavior; a command deadline SHALL be reported as a failure.

#### Scenario: A repository has a multi-megabyte diff

- **WHEN** git emits more than the remaining diff budget
- **THEN** capture stops the producer and returns the bounded prefix with a visible truncation marker
