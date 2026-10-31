## ADDED Requirements

### Requirement: Parent-scoped local HTML images
The iOS HTML preview SHALL display declared local image sources and responsive image candidates through opaque native resource identifiers. The server SHALL verify each image against the immutable HTML parent, session/home, supported image type and directory boundary. Existing resource budgets, guest restrictions, external-resource isolation and script consent SHALL remain.

#### Scenario: A report uses a relative encoded image filename
- **WHEN** a verified local HTML report declares an image inside its directory tree
- **THEN** the paired iOS preview loads and decodes it without enabling document scripts or exposing an application bridge
- **AND** undeclared, remote, cross-session and escaping references are refused
