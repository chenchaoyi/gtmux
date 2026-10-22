## Why

Release discovery and installation rely on GitHub availability. Anonymous API quotas
can be shared with unrelated clients, and the raw installer URL is hard to remember.
Provide official domain-based distribution while keeping the same reviewed release
bytes, independent verification and GitHub archive.

## What Changes

Keep builds/signing/notarization in the existing GitHub release workflow. Only after
both CLI and app jobs pass, sign a bounded release manifest and publish the same
assets to a separate R2 bucket. An isolated Pages advanced-mode download service
supports GET/HEAD/conditional/range reads and authenticated immutable publication;
it never uses the private phone distribution bucket. The publisher verifies every
object before atomically advancing stable metadata. Failed verification stops.

Use RSA/SHA256 signatures so the native Go verifier and stock macOS LibreSSL/Linux
OpenSSL bootstrap can verify the same payload without extra packages. Private keys
and publication credentials remain outside Git. Only public keys, service code,
examples and credential-free workflow/docs belong here.

The website exposes short install paths on both mirrors and describes actual
sources and limits. ccy.pub is static Aliyun hosting with external DNS; the separate
Pages download subdomain supports external CNAMEs without migrating root DNS or
routing package bytes through the Aliyun web server. Existing access policy stays.
App Store phone delivery remains independent; only actually affected metadata is
updated. No phone package for a version-only or Mac-only change.

## Surfaces

- terminal: signed discovery/verification and official mirror-first downloads;
  retain explicit version pins, component receipts and authoritative GitHub fallback.
- menubar: consume the same CLI updater; no second network/installation implementation.
- phone: no app update channel change; review existing setup/support information.
- iPad: same App Store/private delivery boundaries as Phone.
- Web: short installers and accurate landing/reference information on both mirrors;
  separate public package endpoint, no private links or credentials.
