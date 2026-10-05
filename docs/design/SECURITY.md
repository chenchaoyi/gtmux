# The trust boundary of gtmux remote access (Security model)

> Decision D1=(a) (2026-06-28): keep the "single-layer TLS tunnel + bearer token" design, **no end-to-end
> encryption**, but state the trust boundary clearly. QR pairing / E2E go on the backlog (see `DECISIONS-FOR-CCY.md` D1).

That paragraph records the June decision. One-time-code pairing has since shipped;
ephemeral-key pairing and application-layer E2E encryption remain unimplemented.

gtmux's remote features let a phone or browser see, and drive, the tmux sessions on your Mac from across
the network. That is an exposure by nature. This document covers what the token means, what the tunnel can
see, and how to keep the risk within what you can accept.

## 1. token = password (the single most important line)

The **master bearer token of `gtmux serve`, or an owner-paired device's token, can execute commands on your Mac**:
`POST /api/send` feeds its content into a pane via `tmux send-keys`, and that is remote code execution (RCE).

- Treat it like a password: a leak means someone else can run commands on your machine.
- The master token lives in `~/.config/gtmux/serve-token` (`0600`); never paste it into chat, screenshots or issues.
- Current pairing prefers a one-time code on both LAN and tunnel routes (see §3).
  Legacy v1 QR payloads contain a lasting bearer token; the menu-bar pairing sheet
  can fall back to that format if code minting fails. A displayed QR is therefore
  not always a credential that expires after five minutes. `gtmux pair` requires
  a fresh code and reports a minting failure instead.

## 2. What the tunnel can see (single-layer TLS, no E2E)

The Standard Cloudflare tunnel runs over this path:

```
手机 ──TLS──> Cloudflare 边缘 ──加密隧道──> 你 Mac 上的 cloudflared ──loopback──> gtmux serve(127.0.0.1)
```

(phone → TLS → Cloudflare edge → encrypted tunnel → cloudflared on your Mac → loopback → gtmux serve on 127.0.0.1)

- TLS terminates at the Cloudflare edge.
  In other words, **Cloudflare can see plaintext API traffic at the edge** (pane contents, the input you send).
  There is currently no application-layer end-to-end encryption: trusting this path means trusting
  Cloudflare plus the hosted control plane (`api.gtmux.ccy.dev`).
- For Standard, the control plane provisions or repairs the Mac's registered
  tunnel and returns its connector token. It does not proxy session traffic;
  that goes through the Cloudflare tunnel itself. The edge can see plaintext
  on this TLS-terminating proxy path.
- Direct/self-hosted access uses a different path: the client connects over HTTPS
  to the selected server's reverse proxy, then through chisel to the Mac. That
  server operator can see the API traffic too; changing the tunnel backend does
  not add application-layer E2E encryption. See [self-hosted tunnel setup](../../deploy/self-tunnel/README.md).
- Push is a separate path through the configured APNs relay and Apple. The relay
  receives notification content and metadata, and Live Activity updates when enabled.
  Turning off a Mac's notifications must reach that Mac before it stops forwarding;
  an offline Mac can leave the app's change pending.
- This is the deliberate trade-off of D1=(a). Removing "the edge sees plaintext" requires the E2E of §5 (not built).

## 3. A one-time pairing code is not a token

The `…/#c=<code>` in a browser / phone pairing link:

- expires after five minutes, one redemption, or a serve restart. While valid,
  it can enroll an owner device with full control: keep it private too.
- sits in the URL fragment (after `#`), which the browser does not send to the server; only the front-end
  JS reads it to exchange it for this device's own per-device token.
- after pairing with it, revoking one device does not affect the others (the master token stays valid).

## 4. Practical steps that bring the risk down to acceptable

- Local network mode sends session API traffic directly to the Mac; push forwarding
  is independent and may still use the hosted relay.
- Check the menu bar's remote-access mode or `gtmux tunnel --status`. To stop the
  background remote-access services, use `gtmux tunnel --unservice`; a tunnel
  running in a foreground terminal must be stopped there too.
- Lost your phone? Revoke that device (per-device tokens are revocable).
- To self-host the Standard control plane, set `GTMUX_TUNNEL_API` to your Worker's
  URL and `GTMUX_TUNNEL_REG` to its registration gate value. Also set
  `GTMUX_TUNNEL_API_FALLBACK` to that same URL, or to your own fallback: changing
  only the primary URL leaves the hosted fallback enabled. For push, configure
  serve's `--relay-url` and `--relay-token`. See the [provisioner README](../../tunnel-worker/README.md)
  and [relay README](../../relay/README.md).
- Never put tokens or other sensitive data in a URL query.

## 5. Future (backlog, not built)

- Ephemeral-key pairing (D1 option b). One-time enrollment codes are already implemented;
  they reduce credential lifetime but do not encrypt API traffic end to end.
- Application-layer end-to-end encryption + zero-knowledge relay (D1 option c): keep session and notification content encrypted across the
  intermediaries described in §2. This would require a new protocol and matching clients;
  it is not a property of the current tunnel or push relay.

## 6. Existing provisioner safeguards

The Standard provisioner already has best-effort per-IP/global creation caps and
a scheduled unused-tunnel reaper; see the [provisioner README](../../tunnel-worker/README.md)
for thresholds and retention boundaries. `x-gtmux-reg` remains a soft gate, not a private client secret.
