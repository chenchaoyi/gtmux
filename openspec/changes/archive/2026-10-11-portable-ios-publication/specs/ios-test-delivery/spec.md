## ADDED Requirements

### Requirement: Publisher setup is portable and checked before a publication build

When test delivery needs hosted publication, maintainers SHALL check local publisher
settings before an expensive archive. A reachable direct installation SHALL NOT
require publication credentials. A fresh machine SHALL support an explicit config
path, BUILDS_PUBLISHER_CONFIG, XDG_CONFIG_HOME/default HOME location, or complete
local/CI environment settings. Paths to the ccy.dev uploader and its Python runtime
SHALL be selected on the current machine, not copied as another user's absolute paths.
Preflight SHALL be local and SHALL NOT claim remote authorization, signing or device
acceptance. Missing setup SHALL identify the prerequisite without printing values or
creating credentials, resources or remote access.

#### Scenario: A different HOME has no publisher file

- **WHEN** a release operator runs preflight on a fresh machine
- **THEN** missing field names and initialization guidance are reported before archiving
- **AND** existing configuration is not assumed to arrive through Git or HQ

#### Scenario: Complete CI secrets are available

- **WHEN** the five publisher settings are supplied through the environment
- **THEN** the adapter works without a configuration file and does not put values in arguments

### Requirement: Configuration transfer and uploader output stay private

Configuration SHALL be obtained only over an existing verified SSH or authorized
secret channel for a trusted release operator. The runbook SHALL NOT claim that an
unverified source host is reachable. Stdin import SHALL validate bounded JSON,
permit only documented fields, create a mode-0600 file and refuse overwrite,
symlinks, malformed or incomplete input. File loading SHALL require owner ownership
and mode 0600. The adapter SHALL delegate IPA publication and link issuance to the
existing ccy.dev uploader with credentials in its environment. Both stdout and stderr
SHALL be written to new mode-0600 private paths, never echoed to public logs; failures
SHALL retain their exit status and existing IPA/receipt. Testers SHALL receive only
their scoped installation link, not publisher credentials.

#### Scenario: SSH fails or returns incomplete input

- **WHEN** stdin import has no valid complete publisher configuration
- **THEN** no configuration is created and an existing file is not modified

#### Scenario: Publication fails after writing private output

- **WHEN** the delegated uploader exits unsuccessfully
- **THEN** its exit status is preserved and private output remains for diagnosis
- **AND** a retry uses new output paths and the same validated IPA/receipt

### Requirement: Open-source hosted delivery is explicitly trusted and private

A hosted test-package workflow SHALL be manual and disabled until operator setup.
It SHALL run from the default main branch. Release-test mode SHALL accept only a
published stable tag whose resolved commit belongs to main and whose latest main
CI passed. UI-preview mode SHALL accept an exact main commit with latest main CI
passed without requiring or creating a release tag, GitHub Release or ASC submission.
It SHALL retain the source App version, use a distinct workflow build number and
label installation metadata/notes with UI-preview purpose and commit identity.
Before expensive building it SHALL validate the required setup. A protected
maintainer environment SHALL hold iOS distribution signing and publisher secrets;
fork and PR runs SHALL NOT enter that workflow or receive those credentials.
The private uploader SHALL be checked out at a pinned revision with read-only
access, not copied into the public repository. Export SHALL use Ad Hoc profiles
for the App and both extensions with matching team, valid expiry, registered
common devices and production APNs. Raw IPA, profiles, credentials, logs and
installer bearer URLs SHALL NOT be public artifacts or summary content. Recovery
artifacts SHALL be encrypted to an operator-held age identity; the private identity
SHALL NOT be sent to GitHub. Credentials SHALL NOT be configured by merging code.

#### Scenario: A fork or unapproved ref requests publication

- **WHEN** hosted delivery is not enabled, runs from a non-main ref, or targets an unchecked commit or an unpublished release-test tag
- **THEN** no signing, build or publication job proceeds

#### Scenario: Publication or export fails

- **WHEN** a trusted runner fails after preparing private output
- **THEN** it reports a failing status without dumping private output
- **AND** any uploaded recovery files are encrypted, with no credentials/profile setup files included

#### Scenario: A tester installs the hosted package

- **WHEN** a maintainer retrieves the encrypted result and shares the scoped installer link
- **THEN** no publisher credential is needed on the tester's machine
- **AND** build/publication success is not reported as device installation acceptance

#### Scenario: UI verification needs a package but no release

- **WHEN** a maintainer chooses UI-preview for a CI-green main commit
- **THEN** no release, tag or store metadata is created
- **AND** the Ad Hoc package and its installation notes identify the preview commit/build
