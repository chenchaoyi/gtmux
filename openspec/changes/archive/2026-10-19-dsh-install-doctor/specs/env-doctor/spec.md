## ADDED Requirements

### Requirement: Distinguish DSH installation from runtime loading

Doctor SHALL check the current owned Cordis module and exact launch overlay, rather than accepting marker substrings alone. A healthy installed integration SHALL include the actionable `dsh web --patch` command and state that loading in an already running Web process is unverified. Doctor SHALL NOT restart that process, edit its user profile, infer live loading from saved conversation history or use Codex compatibility hooks to repair the native integration.

#### Scenario: Both files are current but Web predates installation

- **WHEN** doctor observes a current installed module and overlay
- **THEN** it reports files installed with runtime loading unverified and supplies the next-launch command

#### Scenario: A valid module has a malformed overlay

- **WHEN** an overlay contains the plugin id but does not match the generated configuration
- **THEN** doctor reports repair needed rather than a healthy integration
