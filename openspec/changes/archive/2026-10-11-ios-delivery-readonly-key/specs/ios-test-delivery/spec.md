## ADDED Requirements

### Requirement: Private uploader access is a dedicated read-only deploy key

The hosted workflow SHALL check out its pinned private uploader using an environment
SSH secret dedicated to a read-only deploy key on that repository. It SHALL NOT use
a personal broad-access token or the maintainer's login SSH key. Preflight SHALL
require the deploy-key secret before importing signing material or building.

#### Scenario: Only a personal token is configured

- **WHEN** the dedicated uploader SSH key is absent
- **THEN** preflight reports the missing secret name without printing credential values
- **AND** no signing or archive proceeds
