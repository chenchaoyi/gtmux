## ADDED Requirements

### Requirement: Mobile request diagnostics identify and isolate their server
Phone and iPad request failure sampling, repeat counts and recovery SHALL be scoped
to sanitized transport origin plus method/path, including Direct path prefixes.
Success at another origin or route SHALL NOT clear or suppress an outstanding failure.
Unknown-source activity SHALL remain separate from identified origins.
Request failure/recovery descriptions SHALL show the recorded origin; push-registration
receipts SHALL show the sanitized server base address, including its prefix.
Source metadata SHALL exclude URL userinfo, query and fragment credentials.
Existing bounded storage, severity, 304/cancellation semantics and credential redaction
SHALL remain intact. Historical records SHALL NOT be rewritten or assigned guessed sources.
Recovery language SHALL describe the request, not claim the entire host is healthy;
a registration receipt SHALL NOT be treated as proof of APNs notification delivery.
No network request, notification preference or retry policy SHALL change.

#### Scenario: Two hosts share an API path
- **WHEN** both hosts fail POST /api/push/register and only one later succeeds
- **THEN** each failure and repeat count remains independent
- **AND** only the successful host gets a recovery record naming its origin

#### Scenario: Direct prefixes and sensitive URL fields
- **WHEN** a client uses a prefixed server address containing URL userinfo or credentials in query/fragment
- **THEN** the origin and method/path retain the real host and routing prefix
- **AND** source metadata excludes those credentials and push receipts identify the sanitized base

#### Scenario: Historical source was never recorded
- **WHEN** old request records contain only method/path
- **THEN** their original details remain readable without a fabricated server
- **AND** a known host's success cannot recover unknown-source failure state
