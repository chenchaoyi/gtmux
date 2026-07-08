# Security & secret inventory

gtmux is a **public, open-source** repo. The rule is simple:

> **No secret value ever lives in the repo.** Secrets live in Cloudflare Worker
> secrets / KV, the VPS's `EnvironmentFile`, the macOS keychain / `~/.appstoreconnect`,
> GitHub Actions secrets, and the gitignored `macapp/.release.env`. The repo only ever
> holds **references** (env var names, KV bindings, non-secret account/zone IDs).

This file is the map: **where each secret lives** and **how to rotate it**. It contains
no secret values — by design.

## Guards in place

- **GitHub secret scanning + push protection: ON** — a push that contains a *recognized*
  provider token (AWS/Stripe/GitHub/…) is blocked before it reaches the remote.
  ⚠️ Push protection does **not** recognize gtmux's *custom* secrets (the 48-hex
  reg/relay tokens, the `user:pass` chisel auth), so the discipline below still matters.
- **`.gitignore`** blocks the secret-bearing filenames (`.dev.vars`, `*.env`, `*.p8`,
  `*.p12`, `*.pem`, `*.key`, `AuthKey_*`, `id_rsa*`) so a stray file can't be staged.
- Worker secrets are set with `wrangler secret put` (encrypted at Cloudflare), **never**
  in `wrangler.toml` — that file's `[vars]` are plaintext/public, fine for non-secret IDs.

## Inventory

### 1. Baked into the CLI at release (soft, ship-in-binary gates)

| Secret | Where it lives | Purpose |
|---|---|---|
| `GTMUX_TUNNEL_REG` | GitHub Actions secret **and** `macapp/.release.env` (local, gitignored) | hosted-tunnel registration gate (`x-gtmux-reg`) |
| `GTMUX_RELAY_TOKEN` | same two places | bearer the Mac presents to the push relay |

Injected via `-ldflags` in `.goreleaser.yaml` (CI CLI) and `macapp/build.sh` (local app).
These are **soft** gates — they necessarily ship in every binary; treat as
anti-abuse, not real secrets. **Rotate:** change the value in the GitHub secret **and**
`macapp/.release.env`, update the matching Worker secret (below), then cut a release.

### 2. tunnel-worker (Cloudflare Worker `api.gtmux.ccy.dev`)

| Name | Kind | Purpose |
|---|---|---|
| `CF_API_TOKEN` | wrangler **secret** | drives the Cloudflare API (tunnel + DNS) |
| `REG_SECRET` | wrangler **secret** | must equal `GTMUX_TUNNEL_REG` above |
| `DIRECT_URL` | wrangler **secret** | the Direct server base returned on a valid code |
| `DIRECT_SECRET` | wrangler **secret** | the Direct chisel auth returned on a valid code |
| `TUNNELS`, `DIRECT_CODES` | **KV** | deviceId→tunnel records; valid Direct access codes |
| `CF_ACCOUNT_ID`, `CF_ZONE_ID`, `ZONE_NAME`, `LOCAL_SERVICE` | `[vars]` (non-secret) | IDs / config |

**Rotate a secret:** `cd tunnel-worker && npx wrangler secret put <NAME>`.
**Issue / revoke a Direct code:** `./tunnel-worker/issue-direct-code.sh "<buyer>"` /
`npx wrangler kv key delete --binding DIRECT_CODES <code> --remote`.

### 3. relay-worker (Cloudflare Worker — push relay)

| Name | Kind | Purpose |
|---|---|---|
| `RELAY_TOKEN` | wrangler **secret** | must equal `GTMUX_RELAY_TOKEN` above |
| `APNS_KEY` | wrangler **secret** | the APNs `AuthKey_*.p8` PEM contents |
| `APNS_KEY_ID`, `APNS_TEAM_ID`, `APNS_TOPIC`, `APNS_ENV` | `[vars]` (non-secret) | APNs routing |

**Rotate:** `cd relay-worker && npx wrangler secret put <NAME>` (then `npx wrangler deploy`).

### 4. DMIT VPS (the Direct / self-hosted tunnel server)

| Secret | Where it lives | Purpose |
|---|---|---|
| chisel `AUTH` (`user:pass`) | `/etc/gtmux-tunnel/chisel.env` on the VPS (root, 0600) | authenticates the Mac's chisel client |

Must match the Worker's `DIRECT_SECRET`. **Rotate:** update `/etc/gtmux-tunnel/chisel.env`
+ the Worker's `DIRECT_SECRET`, restart `chisel-server`, and have Direct users re-run
`gtmux tunnel --redeem <code>` (they'll pull the new secret). SSH key for the VPS is a
local `id_rsa` (never in the repo).

### 5. macOS signing & notarization (release only)

- **Developer ID Application cert** — the login keychain (local), or GitHub secrets
  `MACOS_CERT_P12` / `MACOS_CERT_PASSWORD` (CI path).
- **App Store Connect API key** — `~/.appstoreconnect/private_keys/AuthKey_*.p8` +
  a `notarytool store-credentials` keychain profile (local), or GitHub secrets
  `MACOS_NOTARY_KEY_P8` / `MACOS_NOTARY_KEY_ID` / `MACOS_NOTARY_ISSUER` (CI path).

See `docs/release-signing.md`. None of these live in the repo.

## If a secret ever lands in a commit

A secret pushed to a **public** repo is compromised within seconds (bots scrape
continuously). **Rotate first, purge history second:**

1. **Rotate immediately** using the steps above — this is the real fix.
2. Then scrub git history (`git filter-repo` / BFG) if you want it gone, and
   force-push. History rewrite alone is **not** a substitute for rotation.

## Where NOT to put secrets

- Not in `wrangler.toml` `[vars]` (plaintext, public) — use `wrangler secret put`.
- Not in source, tests, comments, or committed config. Local dev → `.dev.vars`
  (gitignored). CI → GitHub Actions secrets. Release → `macapp/.release.env` (gitignored).
