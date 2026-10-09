## ADDED Requirements

### Requirement: Browser entry preserves its Mac path prefix

The browser mirror SHALL load its scripts, styles, fonts and API calls inside the Mac path prefix even when a share URL has no trailing slash. Its asset base SHALL be set before any external asset reference, remain on the current origin, and exclude query strings and credential fragments. Root and explicit index.html entries SHALL use the same API prefix as the document base.

#### Scenario: A guest opens an existing Direct share URL

- **WHEN** a browser opens `/p35047#code=<valid-share-code>`
- **THEN** the page loads its assets from `/p35047/`, removes the fragment before redemption, and redeems through `/p35047/api/enroll`
- **AND** existing scope checks still determine which sessions and inputs the guest can access

#### Scenario: Root or explicit index entry

- **WHEN** the page opens at `/`, `/index.html`, `/p35047/` or `/p35047/index.html`
- **THEN** its asset URLs and API URLs share the correct root or `/p35047` prefix
- **AND** no credential fragment is added to any asset request
