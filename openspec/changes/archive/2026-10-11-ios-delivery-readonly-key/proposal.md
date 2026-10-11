# Dedicated read-only uploader access

## Why
Completing hosted delivery should not require copying a personal GitHub token into
public-repository CI. A repository-specific read-only deploy key can be provisioned
without a browser token-creation flow and cannot write to the uploader repository.

## What Changes
- Replace the optional workflow uploader token with a dedicated SSH deploy key.
- Fail preflight when the dedicated key is absent; do not fall back to personal tokens.
- Update both maintainer runbooks and regression coverage.

## Impact
Maintainer CI only. Terminal, menu bar, phone, iPad and Web behavior is unchanged.
Existing code merge alone still does not enable the workflow or migrate credentials.

## Surfaces

- terminal: maintainer CI setup only; no gtmux command behavior changes.
- menubar: no product changes.
- phone: hosted test signing only; no UI changes.
- iPad: same hosted test delivery; no device acceptance claim.
- Web: existing publication service reused; no website/Worker deployment.
