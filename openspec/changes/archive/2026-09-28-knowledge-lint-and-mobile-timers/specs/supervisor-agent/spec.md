## ADDED Requirements

### Requirement: Knowledge lint SHALL distinguish examples from references and credentials

Knowledge lint SHALL ignore placeholder wiki syntax and bracketed prompt samples.
Its sensitive check SHALL require a credential-shaped value after an assignment.
It SHALL continue reporting unresolved real links and credential-shaped values.

#### Scenario: A lesson quotes an example link or explains a token

- **WHEN** the body contains `[[...]]`, `[[链接]]`, `[[[SYSTEM]]]`, or `token = 新消息`
- **THEN** lint SHALL NOT report a broken link or unmarked credential for that text

### Requirement: A committed knowledge operation SHALL have an audit receipt before rendering

The supervisor SHALL record the operation ID in the event journal as soon as the
knowledge ledger append succeeds, before refreshing derived Markdown views.

#### Scenario: Rendering a derived view fails

- **WHEN** the ledger append succeeds and a derived Markdown render fails
- **THEN** the command SHALL report the render error and the event receipt SHALL retain the operation ID
