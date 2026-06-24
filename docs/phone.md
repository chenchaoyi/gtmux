# Mobile & remote access

<img src="assets/screenshot-detail.png" width="200" align="right" alt="gtmux phone — a pane's live screen + reply" />

The third surface is an iOS app (`mobileapp/`, React Native): the same agent
radar in your pocket, with a lock-screen push the moment an agent needs you or
finishes. Read a pane's live screen in color, send a reply or a control key
(`Enter`, `Ctrl-C`, …), attach a screenshot — all gated by a bearer token. It
pairs with `gtmux serve` (HTTP+SSE over your network) and gets push over APNs.

```sh
gtmux serve --port 8765          # prints a token + the reachable URL(s)
```

Then pair the app — scan the menu-bar app's pairing QR, or enter the host + token
manually. You can save several servers and switch between them from the
connection page (tap the server name in the radar header).

<img src="assets/screenshot-servers.png" width="220" alt="gtmux connection page — saved servers, switch / add / remove" />

Two facts decide what you can do from where:

- **Push reaches you anywhere.** Alerts arrive over APNs on any network (cellular,
  home Wi-Fi), even when the phone can't reach the Mac — Mac at the office, you at
  home, you still get "needs you / finished".
- **The live view (radar / read a pane / focus) needs a network path to the Mac.**
  Same Wi-Fi works directly. Different networks need a tunnel (below).

## From anywhere — Tailscale (recommended)

A private mesh between your devices that ignores corporate Wi-Fi client isolation
and works office↔home.

1. **Mac:** `brew install --cask tailscale` (or the App Store), open it, sign in.
2. **iPhone:** install **Tailscale**, sign in with the **same account**.
3. Get the Mac's Tailscale address: `tailscale ip -4` (a `100.x.y.z`).
4. Pair the app to `http://<that-100.x.y.z>:8765` + the serve token. The live
   view now works from any network.

> **Same Wi-Fi can't reach the Mac?** Corporate/guest Wi-Fi often **isolates
> clients** (phone↔Mac blocked) — Tailscale fixes that. Quick check: open
> `http://<mac-ip>:8765/api/health` in the phone's browser; if it doesn't load,
> you need Tailscale (or a tunnel).
>
> **Mainland China:** Tailscale is a VPN-category app and is generally **not in
> the China App Store**. Install it with a non-mainland Apple ID, **or** skip the
> VPN app entirely with the tunnel below (the phone connects to a normal
> `https://…` URL — no VPN app needed).

## From anywhere — `gtmux tunnel` (no VPN app)

An **outbound** reverse tunnel on the Mac: it dials out to a rendezvous point, so
there's no inbound port to open and NAT is no problem. The tunnel client
(`cloudflared`) runs only on the Mac — the mobile app is unchanged (it still pairs
to a `{url, token}`).

```sh
gtmux tunnel            # default: a STABLE hosted address — pair once
gtmux tunnel --quick    # account-less ephemeral URL (changes each run)
```

It starts the read-only radar (if not already up), opens the tunnel, and prints
the public URL + the serve token + a scannable pairing QR. Open the mobile app →
**Add a server → Scan** → connected from any network. (Missing `cloudflared`? It
offers to `brew install` it.)

- **Hosted (default)** gives each Mac a stable `https://gtmux-<id>.ccy.dev`
  address via gtmux's control plane, so the phone **pairs once** and keeps working
  across restarts — the URL never changes. No account or domain on your side.
- **`--quick`** needs no infrastructure but the `trycloudflare.com` URL **rotates
  each run** (re-pair every time) — fine for a quick look, not for "leave it
  running and check later".
- **Self-host:** point `gtmux tunnel` at your own control-plane Worker with
  `GTMUX_TUNNEL_API` / `GTMUX_TUNNEL_REG`. See
  `design/remote-access-tunnel.md` and `../tunnel-worker/`.

## Security

The remote surface is read-only **except `POST /api/send`** (terminal input via
`tmux send-keys`), and everything is gated only by the bearer token. With a public
tunnel URL, that token is the *only* gate (no VPN layer in front): no token → 401,
but **treat the URL + token like a password** — anyone who has both can type into
your Mac. Don't screenshot the pairing QR into a shared channel.

See `../api/contract.md` and `../mobileapp/SPEC.md` for the full protocol.
