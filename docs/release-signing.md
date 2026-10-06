# Release signing & notarization (macOS app) — one-time setup

The tagged release workflow is set up for **Developer ID signing + notarization**. A local `make app`
without `GTMUX_SIGN_ID`, or a CI snapshot without signing credentials, instead uses
ad-hoc signing; that build is not a notarized distributable. This guide covers the
credentials and checks for the release path.

Two ways to do it. Both need the same one-time credentials (§1 cert + §2 API key):

- **CI (the path in use):** the macOS runner signs, notarizes and uploads the app.
  On a tag it checks all five signing secrets (§3) before touching a keychain and
  fails with the missing names. It sets `GTMUX_REQUIRE_NOTARIZE=1`, so the build
  fails if notarization cannot run; `xcrun stapler validate` also checks the app
  before it is zipped and uploaded. The cask is updated when `HOMEBREW_TAP_TOKEN`
  is configured. Supply the build gates below too. The app job depends on the CLI
  job, so the CLI release may already exist when app signing fails; inspect both
  jobs and the app artifact before calling the release complete.
  Before the 2026-10-06 gate fix, missing Key ID or Issuer could skip notarization;
  `internal/releasecheck` now exercises these failure paths with command stubs.
- **Local (manual fallback):** notarize from your Mac with `make app-release` (see
  "Local release" below) — for a CI outage or a hotfix. It accepts the API-key
  environment variables or a keychain profile; the latter can stall if the login keychain is locked in a
  non-interactive shell (as happened on v0.23.0 — CI avoids that failure mode).

## Local release — manual fallback

The normal path is CI (§3–§4). Use this only when CI can't (outage, hotfix from a
Mac). One-time, after making the cert (§1) and API key (§2):

```sh
# store the notary key in a keychain profile named gtmux-notary (no GitHub secrets)
xcrun notarytool store-credentials gtmux-notary \
  --key ~/Desktop/AuthKey_XXXXXXXXXX.p8 --key-id XXXXXXXXXX --issuer 'REPLACE_WITH_ISSUER_ID'
```

The release owner must ensure that CI and the manual path are not publishing the
same app concurrently: both upload with `--clobber` and update the same cask.
Missing any of the five CI signing secrets fails the tagged app job; this is
not a switch that disables CI publication for the manual path.
For an authorized manual release, prepare an annotated tag whose notes contain
the `user:` block (and preferably `user-zh:`), as described in the
[repo guide](../CLAUDE.md#build--verify). Replace the placeholders below:

```sh
release_tag='vX.Y.Z'                 # choose the new release version
release_notes='/path/to/release-notes.txt'
git tag -a "$release_tag" -F "$release_notes"
git push origin "$release_tag"

# Wait for the CLI release and resolve the CI/manual publishing ownership above.
# This read must succeed before running the publisher.
gh release view "$release_tag" || exit 1

# release.sh builds the CURRENT checkout; the tag argument only names the release.
test -z "$(git status --porcelain)" || exit 1
test "$(git rev-parse HEAD)" = "$(git rev-parse "${release_tag}^{commit}")" || exit 1
macapp/release.sh "$release_tag"
```

`make app-release` selects the latest reachable tag; the explicit script argument
above selects the intended release. The script auto-derives the signing identity, builds + notarizes + staples,
uploads the zip, and updates the `gtmux-app` cask. It refuses to run (with a clear
error) if the release doesn't exist yet, so running the publisher too early is harmless —
just wait for the release and re-run.

## Baking the tunnel/relay secrets (REQUIRED for the local path)

`make app-release` bundles a `gtmux` CLI *inside* `Gtmux.app`. Set these two service
gates for the release's hosted tunnel and push features; a cask-only installation
may rely on that bundled CLI:

- **`GTMUX_TUNNEL_REG`** — the Standard hosted-tunnel registration gate. Empty
  without a runtime override → hosted mode asks for `--quick` or a configured gate.
- **`GTMUX_RELAY_TOKEN`** — the hosted push-relay bearer. The local release script
  warns if it is empty; it does not reject the build. A custom relay can be configured at runtime.

These service gates ship in the release CLI and the app's bundled CLI. They are
separate from the Apple signing keys. Keep their build configuration out of git,
in **`macapp/.release.env`** (gitignored), or export it for the release process.

**Direct credentials are not baked into release binaries.** Redeeming an access
code obtains the assigned server and device account from the Worker and writes
`~/.config/gtmux/selftunnel.conf`. `GTMUX_SELFTUNNEL_URL` and
`GTMUX_SELFTUNNEL_SECRET` are runtime overrides for the tunnel process, not release
build settings. A self-hosted server can also use that config file; see
[the self-tunnel guide](../deploy/self-tunnel/README.md).

```sh
# macapp/.release.env  (gitignored)
GTMUX_TUNNEL_REG='REPLACE_WITH_REGISTRATION_GATE'
GTMUX_RELAY_TOKEN='REPLACE_WITH_RELAY_TOKEN'
```

`release.sh` sources this and **refuses to build** if `GTMUX_TUNNEL_REG` is empty.
Get the values from the release owner's maintained configuration;
matching strings in a binary does not establish which service a value belongs to.
The corresponding GitHub Actions secrets are `GTMUX_TUNNEL_REG` and `GTMUX_RELAY_TOKEN`.

## 1. Developer ID Application certificate → `MACOS_CERT_P12` + password

Needs Apple Developer Program membership and a Developer ID Application identity
for the team publishing the app, including its private key.

1. Create the cert: **Xcode → Settings → Accounts → (your team) → Manage
   Certificates → + → Developer ID Application** (or developer.apple.com →
   Certificates → + → Developer ID Application).
2. Export it WITH its private key: **Keychain Access → My Certificates →** the
   "Developer ID Application: …" entry → right-click → **Export → .p12**, set an
   export password.
3. Base64 it for the secret:
   ```sh
   base64 -i DeveloperID.p12 | pbcopy   # → paste into MACOS_CERT_P12
   ```
   - `MACOS_CERT_P12` = that base64
   - `MACOS_CERT_PASSWORD` = the .p12 export password

   The signing identity string is **auto-derived** from the cert in CI — no separate
   secret. (Locally you'd read it with `security find-identity -v -p codesigning`.)

## 2. App Store Connect API key (for notarytool) → `MACOS_NOTARY_*`

1. **App Store Connect → Users and Access → Integrations → App Store Connect API →
   Team Keys → +**. Role: **Developer** (or App Manager). Name it e.g. `gtmux-notary`.
   Creating a team key requires Account Holder or Admin access; see
   [Apple's API-key instructions](https://developer.apple.com/help/app-store-connect/get-started/app-store-connect-api).
2. **Download the `.p8` once** (you can't re-download it). Note the **Key ID** and,
   at the top of the Keys page, the **Issuer ID**.
3. Base64 the .p8:
   ```sh
   base64 -i AuthKey_XXXXXXXXXX.p8 | pbcopy   # → paste into MACOS_NOTARY_KEY_P8
   ```
   - `MACOS_NOTARY_KEY_P8` = that base64
   - `MACOS_NOTARY_KEY_ID` = the Key ID
   - `MACOS_NOTARY_ISSUER` = the Issuer ID

## 3. Add the five secrets

**GitHub → the gtmux repo → Settings → Secrets and variables → Actions → New
repository secret**, for each of:
`MACOS_CERT_P12`, `MACOS_CERT_PASSWORD`, `MACOS_NOTARY_KEY_P8`,
`MACOS_NOTARY_KEY_ID`, `MACOS_NOTARY_ISSUER`.

These five are for signing. The full release also uses the two build gates above
and `HOMEBREW_TAP_TOKEN` for cross-repository cask publication; signing configuration
alone does not configure those services.

## 4. Verify

After the authorized release runs, confirm the app job logs show Developer ID
signing, a notarization submission, `notarized + stapled`, and the separate tag
validation step before ZIP/upload. Download and inspect
the actual release artifact; a configured signing step alone is not proof of
notarization. On a test Mac, use the same installation path for installation and checks:

```sh
brew install --cask --appdir="$HOME/Applications" chenchaoyi/tap/gtmux-app
codesign --verify --strict --verbose=2 "$HOME/Applications/Gtmux.app"
xcrun stapler validate "$HOME/Applications/Gtmux.app"
spctl -a -vvv "$HOME/Applications/Gtmux.app"  # expect accepted / Notarized Developer ID
open "$HOME/Applications/Gtmux.app"
```

Homebrew's default app directory is `/Applications`; the explicit
[`--appdir`](https://docs.brew.sh/Manpage#global-cask-options) above chooses the user directory.
Record the app version, macOS version and results separately. Notarization checks
do not exercise the menu bar, Automation permissions or other runtime behavior.

## Local signing (optional)

`GTMUX_SIGN_ID="Developer ID Application: …" GTMUX_NOTARY_PROFILE='gtmux-notary' macapp/build.sh`
signs + notarizes locally too (store the profile once with
`xcrun notarytool store-credentials`).

Alternatively, provide all three API-key variables: `GTMUX_NOTARY_KEY` (path to
the `.p8`), `GTMUX_NOTARY_KEY_ID` and `GTMUX_NOTARY_ISSUER`. `release.sh` prefers
that complete triple over a profile; when calling `build.sh` directly, a supplied
`GTMUX_NOTARY_PROFILE` takes precedence. Apple documents the authentication options
in [the notarization workflow](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow).
