# The trust boundary of gtmux remote access (Security model)

> Decision D1=(a) (2026-06-28): keep the "single-layer TLS tunnel + bearer token" design, **no end-to-end
> encryption**, but state the trust boundary clearly. QR pairing / E2E go on the backlog (see `DECISIONS-FOR-CCY.md` D1).

gtmux's remote features let a phone or browser see, and drive, the tmux sessions on your Mac from across
the network. That is an exposure by nature. This document covers what the token means, what the tunnel can
see, and how to keep the risk within what you can accept.

## 1. token = password (the single most important line)

The **bearer token of `gtmux serve` / `gtmux tunnel` is a key that can execute commands on your Mac**:
`POST /api/send` feeds its content into a pane via `tmux send-keys`, and that is remote code execution (RCE).

- Treat it like a password: a leak means someone else can run commands on your machine.
- The token lives in `~/.config/gtmux/serve-token` (`0600`); never paste it into chat, screenshots or issues.
- The pairing QR code carries the token for a short window (the LAN direct-connect case); the tunnel path
  uses a one-time pairing code rather than the token (see §3).

## 2. What the tunnel can see (single-layer TLS, no E2E)

The hosted "anywhere" tunnel (`gtmux tunnel`, Anywhere mode) runs over this path:

```
手机 ──TLS──> Cloudflare 边缘 ──加密隧道──> 你 Mac 上的 cloudflared ──loopback──> gtmux serve(127.0.0.1)
```

(phone → TLS → Cloudflare edge → encrypted tunnel → cloudflared on your Mac → loopback → gtmux serve on 127.0.0.1)

- TLS terminates at the Cloudflare edge (single-layer Universal SSL, covering `ccy.dev`/`*.ccy.dev`).
  In other words, **Cloudflare can see plaintext API traffic at the edge** (pane contents, the input you send).
  There is currently no application-layer end-to-end encryption: trusting this path means trusting
  Cloudflare plus the hosted control plane (`api.gtmux.ccy.dev`).
- The control plane only "issues one named per-Mac tunnel keyed by deviceId and returns the connector token";
  it does not proxy your session traffic (that goes through the Cloudflare tunnel itself), but "the edge sees
  plaintext" holds for any reverse-proxy tunnel.
- Push goes through the hosted APNs relay, which can see notification content (agent name / task
  text). Turn push off if you do not want that exposed.
- This is the deliberate trade-off of D1=(a). Removing "the edge sees plaintext" requires the E2E of §5 (not built).

## 3. A one-time pairing code is not a token

The `…/#c=<code>` in a browser / phone pairing link:

- is a one-time, 5-minute, single-use enroll code, with none of the reach of the long-lived token.
- sits in the URL fragment (after `#`), which the browser does not send to the server; only the front-end
  JS reads it to exchange it for this device's own per-device token.
- after pairing with it, revoking one device does not affect the others (the master token stays valid).

## 4. Practical steps that bring the risk down to acceptable

- On a trusted network, prefer Wi-Fi mode (`serve`, LAN direct connection, nothing goes through the
  cloud); save the Anywhere tunnel for when you are really out.
- Anywhere is a long-lived exposure: the menu bar shows a green "remote on" indicator, and since v0.11.4 a
  "a device is viewing" indicator too, so it never sits there silently. When you are done, `gtmux tunnel --unservice`.
- Lost your phone? Revoke that device (per-device tokens are revocable).
- Don't trust the ccy.dev hosted control plane / relay? Self-host: point `GTMUX_TUNNEL_API` /
  `GTMUX_TUNNEL_REG` at your own Worker; same for the relay. (See `docs/design/remote-access-tunnel.md`,
  [[hosted-tunnel-a1]])
- Default to the most private choice: refuse non-essential cookies/consent; never put sensitive data in a URL query.

## 5. Future (backlog, not built)

- QR pairing + ephemeral key pair (D1 option b): stop putting the token in plaintext inside QR codes / links.
- Application-layer end-to-end encryption + zero-knowledge relay (D1 option c): make sure neither the
  Cloudflare edge nor the relay can see plaintext (what Happy does). This is the only way to remove the
  "edge sees plaintext" of §2.
- Tunnel abuse hardening (per-device caps / reclamation / rate limits). Today `x-gtmux-reg` is only a soft gate.
