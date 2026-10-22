# Official downloads and release hosting

[中文](release-hosting.zh.md)

## User paths

The short installer is `https://ccy.dev/gtmux/install`, with `https://ccy.pub/gtmux/install` as the other entry. These are static copies of `bootstrap.sh`. They require Bash and OpenSSL/LibreSSL, verify signed metadata using the embedded public key, pin a version, verify the installer’s SHA256 and size, then run that installer. An older/unmirrored explicit pin may use the verified current installer while keeping its requested GitHub artifact version. Fetching the initial bootstrap still trusts the selected website’s HTTPS and deployed code.

The default native updater and installer prefer `downloads.ccy.dev`, then `downloads.ccy.pub`. They use a signed `gtmux/stable.json` index and immutable `gtmux/vX.Y.Z/manifest.json` metadata. Four CLI archives, the notarized Mac app, CLI checksums and the installer are the only permitted files. Metadata records the exact binary source and the installer source separately, which allows an existing release to gain a reviewed installer without rebuilding or relabelling its binaries.

RSA-3072/SHA256 authenticates the original payload bytes; signed SHA256 and byte counts cover every artifact, including the Mac ZIP. A malformed signature stops the operation. Ordinary HTTP/network unavailability can try another official origin and the existing GitHub channel. Once metadata is verified, any fallback bytes must match it. Official mirror requests do not follow redirects. The native updater never executes a remote script. Explicit repository/mirror overrides retain their previous channel behavior; the short bootstrap always authenticates the official channel.

Direct download links use `https://downloads.ccy.dev/gtmux/latest/macos.zip` or `latest/darwin-arm64.tar.gz` (also `darwin-amd64`, `linux-arm64`, `linux-amd64`). These resolve to immutable version paths. GitHub Releases retain the complete official archives and checksums. Homebrew continues to use its existing GitHub casks.

## Build and publication

GitHub Actions still builds the cgo-free CLI and signs, notarizes and staples the universal Mac app. Only when both exact-source jobs succeed may `release.yml` publish the same bytes. The publisher rechecks GitHub API asset size/digests and the CLI checksum list, signs canonical metadata, uploads immutable objects, verifies both public download routes, then promotes stable last. Missing assets, bad digests, failed signing/jobs, changed version objects, downgrade or concurrent stable changes stop publication. No partial release becomes the stable pointer.

`release-mirror.yml` is a main-only manual backfill of an existing stable tag; it checks the original release jobs and records the current reviewed installer source. It does not build binaries, rewrite tags or replace GitHub artifacts. A failed post-promotion readback must be reported as uncertain delivery, not rolled back by overwriting the immutable release. Diagnose the recorded source and restore the service before retrying.

The isolated Pages advanced-mode service reads a dedicated R2 bucket, supports GET/HEAD, ETags and single byte ranges, and exposes no listing, CORS, cookies or phone/private package paths. Its authenticated publisher accepts only the fixed release filenames. Default Pages hostnames cannot serve public downloads. Publication uses a separate bearer secret; the release job does not receive the Cloudflare account API credential. The example config contains no account identifiers or credentials.

## Operator setup and boundaries

1. Create a dedicated R2 Standard bucket and a Pages project; bind it as `RELEASES`. Use `download-worker/wrangler.example.toml` as an operator-private configuration. Keep the private phone distribution bucket separate.
2. Set the exact download host allowlist and the separate publication hostname. Associate each custom domain with Pages before configuring its subdomain CNAME. An external DNS provider can CNAME a subdomain without migrating its root zone. Preserve the existing website region/auth rules and deny public downloads through the default Pages hostname.
3. Retain the release private key in private storage/recovery material. Install it as GitHub secret `GTMUX_RELEASE_SIGNING_KEY`; install an independent publication bearer as both Pages secret `PUBLISH_TOKEN` and GitHub secret `GTMUX_RELEASE_PUBLISH_TOKEN`. The public verification key alone belongs in Git. Public-key changes require rebuilding consumers and updating the reviewed static bootstrap; do not silently rotate to an unrecognized key.
4. Run `make ci-test`, `python3 scripts/sync-release-feed.py --check`, `npm --prefix download-worker test` and `npm --prefix download-worker run build`, then deploy the exact reviewed source. Do not publish credentials, private recovery files or operator config as CI artifacts. Public scripts, examples, workflow secret names, manifests and public keys are appropriate repository content.
5. Before advertising the short URLs, verify DNS/TLS, actual signed metadata, full artifact hashes on both origins, short-bootstrap bytes, unauthorized publisher denial, default-host denial and browser download links. Code/CI success alone proves none of those endpoints are live.

Current publication is bounded to 90 MiB per file to stay below the default Cloudflare request upload cap; oversized files fail before stable promotion. Native consumers also impose a 256 MiB maximum. R2 egress is free under the published pricing, but storage/operations and Pages Functions requests have separate quotas shared with other account usage. Budget using measured file sizes and real traffic; a free allowance is not verified account headroom. This service uses Functions for R2 reads, so it is not an unlimited static-assets request claim.

The phone app stays on App Store/TestFlight/private signed delivery. Its Store/support/privacy URLs are website links and do not become binary download or publisher links. A hosting-only change is not a reason to rebuild the phone or modify a pending Store submission. Website installation/download text must match the deployed routes; generated documentation keeps its own exact source provenance.

References: [Pages custom domains](https://developers.cloudflare.com/pages/configuration/custom-domains/), [R2 pricing](https://developers.cloudflare.com/r2/pricing/), [Functions pricing](https://developers.cloudflare.com/pages/functions/pricing/), [platform limits](https://developers.cloudflare.com/workers/platform/limits/).
