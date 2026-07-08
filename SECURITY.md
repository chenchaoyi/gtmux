# Security

gtmux is a **public, open-source** repo that also holds the ops code (the Cloudflare
Workers, the VPS deploy configs). The rule is simple:

> **No secret value ever lives in the repo.** Secrets live in Cloudflare Worker
> secrets / KV, the VPS's `EnvironmentFile`, the macOS keychain / `~/.appstoreconnect`,
> GitHub Actions secrets, and the gitignored `macapp/.release.env`. The repo only ever
> holds **references** — env-var names, KV bindings, non-secret account/zone IDs.

## Where secrets live (categories)

| Kind of secret | Home | How it's set / rotated |
|---|---|---|
| Cloudflare Worker secrets (relay/tunnel tokens, CF API token, APNs key, Direct config) | the Worker's encrypted secret store | `wrangler secret put <NAME>` → `wrangler deploy` |
| Worker data (paired devices, access codes) | Cloudflare **KV** | `wrangler kv key put/delete` |
| VPS-side (the chisel tunnel auth) | `/etc/gtmux-tunnel/…` `EnvironmentFile` (root, 0600) | edit on the VPS + restart the unit |
| Release-baked soft gates (reg / relay token) | GitHub Actions secrets **and** `macapp/.release.env` (gitignored) | change both, then re-release |
| macOS signing / notarization | login keychain + `~/.appstoreconnect` (or GitHub secrets in CI) | see `docs/release-signing.md` |

None of these are in the repo. `wrangler.toml` `[vars]` holds only **non-secret**
identifiers (account/zone/KV IDs) — never put a token there; that file is plaintext.

## Guards in place

- **GitHub secret scanning + push protection: ON** — a push containing a *recognized*
  provider token is blocked before it reaches the remote. ⚠️ It does **not** recognize
  gtmux's *custom* secrets (e.g. a `user:pass` chisel auth), so the rule above still matters.
- **`.gitignore`** blocks secret-bearing filenames (`.dev.vars`, `*.env`, `*.p8/*.p12/
  *.pem/*.key`, `AuthKey_*`, `id_rsa*`). NB: `.gitignore` allows no inline comments —
  patterns are one-per-line.

## If a secret ever lands in a commit

A secret pushed to a **public** repo is compromised within seconds (bots scrape
continuously). **Rotate first, purge history second:**

1. **Rotate immediately** (the "how it's rotated" column above) — this is the real fix.
2. Then scrub git history (`git filter-repo` / BFG) and force-push if you want it gone.
   History rewrite alone is **not** a substitute for rotation.

## Reporting

Found a vulnerability? Open a private report via GitHub Security Advisories, or email
the maintainer — please don't file a public issue for anything exploitable.

---

*Maintainers:* the per-secret inventory (which secret maps to what) and operator runbooks
(issuing/revoking paid Direct codes, rotating this instance's infra) are kept **private**
in `ops/` (gitignored) — they're operator-specific, not product docs.
