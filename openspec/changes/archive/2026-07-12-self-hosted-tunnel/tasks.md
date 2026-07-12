## 1. Provider seam (internal/app/tunnel.go)

- [x] 1.1 Extract a `tunnelProvider` interface: `name() string`, `clientArgs(port int) []string`,
      `readyRe() *regexp.Regexp`, `resolveURL(line string) string`, `ensureClient() string`.
- [x] 1.2 Refactor the cloudflared paths (hosted `tunnel run`, quick `--url`) into a
      `cloudflareProvider` implementing the seam — behavior byte-identical to today.
- [x] 1.3 Provider selection: `--backend cloudflare|self` flag + `GTMUX_TUNNEL_BACKEND`
      (default cloudflare). Unknown/absent → cloudflare.

## 2. Self-hosted backend (Chisel)

- [x] 2.1 `chiselProvider`: dial `GTMUX_SELFTUNNEL_URL` (wss://…) over 443 with
      `GTMUX_SELFTUNNEL_SECRET`; the reachable phone URL is the user's own domain.
- [x] 2.2 `ensureChisel()` mirrors `ensureCloudflared()` — detect missing, offer
      `brew install chisel`, else point at the manual install.
- [x] 2.3 Missing self-host config → a clear "set GTMUX_SELFTUNNEL_URL/SECRET" message
      (bilingual, via i18n), not a crash.
- [x] 2.4 `internal/app/tunnelservice.go`: the always-on plist runs the selected backend.

## 3. Docs

- [x] 3.1 `docs/design/remote-access-tunnel.md`: a "Providers" section (cloudflare
      default / self-hosted) + a short VPS setup guide (running the Chisel server on
      443 behind the user's TLS proxy, with the shared secret).
- [x] 3.2 `docs/phone.md` / `docs/cli.md`: note the `--backend self` option briefly.

## 4. Gate

- [x] 4.1 `make check` green; `CGO_ENABLED=0 go build ./cmd/gtmux` passes (stay cgo-free).
- [x] 4.2 Manual: dogfood `gtmux tunnel --backend self` against the user's VPS on the
      hostile office network; confirm the phone pairs to the user's domain.

## 5. Deferred (P2/P3 — spec'd, NOT built in this change)

- [ ] 5.1 P2: pairing QR carries PRIMARY + FALLBACK url; phone tries Cloudflare then
      self-hosted (contract: add an optional fallback field, older apps ignore it).
- [ ] 5.2 P2: auto-failover — when `tunnelEdgeBlocked()` (PR #303) reports the CF edge
      down, bring up / advertise the self-hosted backend.
- [ ] 5.3 P3: paywall gating for the tunnel tier; health monitoring; embed the Chisel
      client to drop the external binary.
