## ADDED Requirements

### Requirement: Nested links and submitted command inspection

Shared App Markdown SHALL preserve link activation inside supported emphasis and pass inherited styling and callbacks to the native range renderer. Literal code and unsafe link schemes SHALL remain inert. Shared Chat and Trace code readers SHALL expose a bounded outline of static command inputs with the corresponding source selectors. They SHALL retain Full call first, original-copy semantics and unchanged recorded tool-call counts. Dynamic expressions SHALL NOT be evaluated, and input outlines SHALL NOT claim execution or success. The browser SHALL use the same generated command analysis.

#### Scenario: A code-mode call includes a file read

- **WHEN** a recorded wrapper includes a static command and a dynamic command expression
- **THEN** the static command can be inspected as input while the dynamic expression remains only in Full call
