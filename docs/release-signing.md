# Release signing & notarization (macOS app) — one-time setup

By default the release ships an **ad-hoc-signed** `Gtmux.app`: it opens on the Mac
that built it, but on any OTHER Mac Gatekeeper blocks it (a teammate must
`xattr -dr com.apple.quarantine ~/Applications/Gtmux.app` first). Set up **Developer
ID signing + notarization** once and every tagged release opens cleanly on any Mac —
`brew install --cask` and go, TCC grants persist across updates.

Two ways to do it. Both need the same one-time credentials (§1 cert + §2 API key):

- **Local (default here):** notarize from your Mac — no GitHub secrets. Store the
  notary key once in a keychain profile, then each release is `make app-release`.
  When CI has no signing secrets it skips the app upload/cask, so local owns it.
- **CI:** add five repo secrets and every tagged release auto-notarizes (§4).

## Local release — the path in use

One-time, after making the cert (§1) and API key (§2):

```sh
# store the notary key in a keychain profile named gtmux-notary (no GitHub secrets)
xcrun notarytool store-credentials gtmux-notary \
  --key ~/Desktop/AuthKey_XXXXXXXXXX.p8 --key-id XXXXXXXXXX --issuer <issuer-id>
```

Then per release, **in this order**:

```sh
# 1. Push the tag — CI (goreleaser) builds+publishes the CLI and CREATES the release.
git tag v0.12.40 && git push origin v0.12.40

# 2. WAIT until goreleaser has CREATED the release (~1 min) before step 3 —
#    make app-release uploads the app INTO that release, so it must exist first.
#    (No cask race: on the local path CI has no signing secrets, so it skips the app
#    upload + cask entirely — make app-release is the only thing that touches them.)
until gh release view "v0.12.40" >/dev/null 2>&1; do sleep 5; done

# 3. Build + notarize the app and publish it to the release + cask.
make app-release
```

`make app-release` auto-derives the signing identity, builds + notarizes + staples,
uploads the zip, and updates the `gtmux-app` cask. It refuses to run (with a clear
error) if the release doesn't exist yet, so running step 3 too early is harmless —
just wait for the release and re-run.

## Baking the tunnel/relay secrets (REQUIRED for the local path)

`make app-release` bundles a `gtmux` CLI *inside* `Gtmux.app`. Two soft gates must be
baked into that CLI at build time — WITHOUT them the shipped app is broken for anyone
who installs only the cask (their app has no `~/.local/bin/gtmux` to prefer, so it
falls back to the bundled CLI):

- **`GTMUX_TUNNEL_REG`** — the hosted-tunnel registration gate. Empty → "Anywhere"
  fails with "hosted mode isn't configured in this build".
- **`GTMUX_RELAY_TOKEN`** — the push-relay bearer. Empty → push notifications are off.

Neither is a *real* secret (both necessarily ship in every released binary — they're
in the goreleaser CLI and the CI app build too), so we keep them out of git but on
disk in **`macapp/.release.env`** (gitignored):

A third pair bakes the **"Direct" tunnel** (the second gtmux-provided tunnel behind
Anywhere; Standard = Cloudflare). Empty → the Anywhere→Direct choice only works if the
user wrote their own `~/.config/gtmux/selftunnel.conf`:

- **`GTMUX_SELFTUNNEL_URL`** / **`GTMUX_SELFTUNNEL_SECRET`** — the Direct server's URL
  and chisel auth (`user:pass`). Never put these in source — the repo is public.

```sh
# macapp/.release.env  (gitignored)
GTMUX_TUNNEL_REG=<value, also GitHub secret GTMUX_TUNNEL_REG / Worker REG_SECRET>
GTMUX_RELAY_TOKEN=<value, also GitHub secret GTMUX_RELAY_TOKEN>
GTMUX_SELFTUNNEL_URL=<value, also GitHub secret GTMUX_SELFTUNNEL_URL>
GTMUX_SELFTUNNEL_SECRET=<value, also GitHub secret GTMUX_SELFTUNNEL_SECRET>
```

`release.sh` sources this and **hard-refuses to build** if `GTMUX_TUNNEL_REG` is empty,
so a local release can never silently ship a broken Anywhere again. Lost the values?
They're recoverable from any reg-baked binary: `strings ~/.local/bin/gtmux | grep -oE
'\b[0-9a-f]{48}\b'` (the tunnel reg is the one sent as the `x-gtmux-reg` header).

## 1. Developer ID Application certificate → `MACOS_CERT_P12` + password

Needs a paid Apple Developer Program membership (team `2337SY8FRT`).

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
   - `MACOS_CERT_PASSWORD` = the .p12 export password (set a simple one; an
     empty-password .p12 is flaky on the CI runner)

   The signing identity string is **auto-derived** from the cert in CI — no separate
   secret. (Locally you'd read it with `security find-identity -v -p codesigning`.)

## 2. App Store Connect API key (for notarytool) → `MACOS_NOTARY_*`

1. **App Store Connect → Users and Access → Integrations → App Store Connect API →
   Team Keys → +**. Role: **Developer** (or App Manager). Name it e.g. `gtmux-notary`.
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

## 4. Verify

Cut a release tag. In the app job log you should see `code signing (Developer ID …)`
then `notarizing (submit + wait, then staple)… notarized + stapled`. Then on a
teammate's Mac:

```sh
brew install --cask chenchaoyi/tap/gtmux-app
open ~/Applications/Gtmux.app        # opens directly — no xattr/right-click
spctl -a -vvv ~/Applications/Gtmux.app   # → "accepted", source "Notarized Developer ID"
```

## Local signing (optional)

`GTMUX_SIGN_ID="Developer ID Application: …" GTMUX_NOTARY_PROFILE=<profile> macapp/build.sh`
signs + notarizes locally too (store the profile once with
`xcrun notarytool store-credentials`).
