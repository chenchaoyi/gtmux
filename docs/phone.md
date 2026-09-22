# Mobile & remote access

**English** · [中文](phone.zh.md)

<img src="assets/screenshot-detail.png" width="200" align="right" alt="gtmux phone: a pane's live screen + reply" />

gtmux has an iOS app: the same agent radar on your phone, with a lock-screen push
the moment an agent needs you or finishes. You can read a pane's live screen in
color, send a reply or a control key (`Enter`, `Ctrl-C`, and so on), and attach a
screenshot. Agents running outside tmux appear read-only under an "Elsewhere"
section, as in the menu bar: they have no pane, so there is nothing to jump to or
reply into.

The app talks to `gtmux serve` on the Mac and receives push notifications through
Apple's notification service.

```sh
gtmux serve --port 8765          # prints a token + the reachable URL(s)
```

Then pair the app: run `gtmux pair` and scan the QR it prints (the menu-bar app
shows the same QR under ⚙︎ → Pair a device…), or enter the address and token by
hand. You can save several Macs and switch between them from the connection page
(tap the server name in the radar header).

## No terminal needed: the menu-bar app has the same controls

<img src="assets/menubar-remote.png" width="418" alt="menu-bar Preferences, Remote access: Off / Local network / Anywhere, tunnel Standard / Direct" />

Everything below about turning on remote access is also two clicks in the
menu-bar app: click the gtmux status icon, then ⚙︎ → Preferences… → Remote
access. That page has the same three-way switch (Off / Local network / Anywhere), the
tunnel type under Anywhere (Standard / Direct), and the reachable address while
remote access is on. ⚙︎ → Pair a device… shows the one-time pairing QR/code
directly, and turns remote access on first if it is off. The Sharing section in
Preferences manages the same guest links as `gtmux share`.

A paired phone (an owner device) can manage sharing remotely. Its Manage this Mac
screen lets you create, copy and revoke the same guest links as `gtmux share`
(per pane: view, type) and shows the list of paired devices, without walking to
the Mac. Two things stay Mac-only: revoking a paired device, and switching remote
access on or off, so a lost phone cannot re-key the machine. A guest connection
never sees this screen.

<img src="assets/screenshot-servers.png" width="220" alt="gtmux connection page: saved servers, switch / add / remove" />

Two facts decide what works from where:

- Push reaches you anywhere. Alerts arrive on any network (cellular, home Wi-Fi),
  even when the phone cannot reach the Mac. Mac at the office, you at home: you
  still get "needs you" and "finished".
- The live view (the radar, reading a pane, focus) needs a network path to the
  Mac. On the same local network it works directly. From a different network you need
  remote access, set up below.

## From anywhere: `gtmux tunnel` (recommended)

The Mac opens an outbound tunnel, so there is no inbound port to open and NAT does
not matter. Only the Mac runs the tunnel client (`cloudflared`); the phone just
opens a normal `https://…` address.

```sh
gtmux tunnel                  # Standard: a stable hosted address, pair once
gtmux tunnel --backend self   # Direct: through gtmux's own server (paid; see --redeem)
gtmux tunnel --quick          # account-less ephemeral URL (changes each run)
gtmux tunnel --service        # keep it on across reboots (--unservice / --status)
```

It starts the radar server if it is not already up, opens the tunnel, and prints
the public address, the token and a pairing QR, plus an "open on computer" link to
a read-only web mirror (view the radar and a pane in a browser, no app needed). In
the mobile app, go to Add a server → Scan, and you are connected from any network.
If `cloudflared` is missing, it offers to `brew install` it.

Anywhere comes in two kinds:

- Standard (default): a free, zero-config tunnel. Each Mac gets a stable
  `https://<id>.gtmux.ccy.dev`, so the phone pairs once and keeps working across
  restarts. No account or domain on your side.
- Direct (`--backend self`): a tunnel over port 443 through gtmux's own server,
  for restrictive networks that block the Standard tunnel (some corporate
  networks). It is a paid unlock: get an access code at
  <https://ccy.dev/projects/gtmux/direct>, redeem it with
  `gtmux tunnel --redeem <code>` (or the menu bar's Anywhere → Direct, which
  prompts for one), then use `--backend self`. Each Mac gets its own address,
  `https://tunnel.ccy.dev/p<port>`, and its own account on the server, which can only
  ever reach that address; one code unlocks up to three Macs, and redeeming again on the
  same Mac is fine. To run your own server instead, point at it
  with `GTMUX_SELFTUNNEL_URL` + `GTMUX_SELFTUNNEL_SECRET`; the setup lives in
  `deploy/self-tunnel/` in the repo.
- `--quick`: no setup at all, but the `trycloudflare.com` address changes on every
  run, so you re-pair every time. Fine for a quick look, not for leaving it
  running.

Keep it on across reboots: `gtmux tunnel --service` (or the menu-bar Anywhere
toggle) registers it as a background service; `--unservice` turns it off,
`--status` shows the state. A MacBook with its lid closed goes to sleep and the
tunnel drops with it; `gtmux awake on` keeps the Mac, the tunnel and the phone
answering with the lid shut (`gtmux awake off` needs no password; see
[`cli.md` → `gtmux awake`](cli.md)).

Contributors who want to host the tunnel service themselves: `GTMUX_TUNNEL_API` /
`GTMUX_TUNNEL_REG` point `gtmux tunnel` at your own instance; see
[`design/remote-access-tunnel.md`](design/remote-access-tunnel.md).

## From anywhere: Tailscale or any VPN

If you already run Tailscale (or another VPN) between your devices, that works
too, and it also gets around corporate Wi-Fi client isolation. Install Tailscale
on the Mac (`brew install --cask tailscale`, or the App Store) and on the iPhone,
sign in with the same account on both, get the Mac's address with
`tailscale ip -4` (a `100.x.y.z`), and pair the app to `http://<that address>:8765`
plus the serve token. Nothing else changes.

> Same Wi-Fi, but the phone cannot reach the Mac? Corporate and guest Wi-Fi often
> isolate clients from each other. Quick check: open
> `http://<mac-ip>:8765/api/health` in the phone's browser; if it does not load,
> use `gtmux tunnel` (or a VPN).

## On an iPad: a sidebar beside the work

The same app, installed from the same App Store listing. On a window at least
768×600 points (any iPad orientation, a 2/3 Split View, a Stage Manager window that
size) the radar becomes a sidebar and whatever you open (a session, gtmux HQ, All
panes) fills the main pane beside it. Nothing is pushed; tap another row and the
main pane switches. Narrower than that (a 1/2 Split View, Slide Over) it is the
phone's layout.

- The sidebar hides with the button next to the gear, or ⌃⌘S, and remembers.
- Chat reads at a comfortable width; the terminal uses the whole pane.
- The HQ page shows its conversation and, beside it, what is waiting on you and what HQ did.
- HQ's knowledge base (the notes it has collected) opens its list beside the entry you are reading.

With a hardware keyboard, hold ⌘ to see the commands. The ones worth learning:

| Keys | What |
|---|---|
| ↑ ↓ ⏎ | move along the radar, open the selected session |
| ⌘1 … ⌘9 | open the nth row |
| ⌘⇧H · ⌘⇧P | gtmux HQ · All panes |
| ⌘F | search panes |
| ⌘K | type a message |
| ⌘[ · ⌘] | chat · terminal |
| ⌘= · ⌘− | text size |
| esc | close a sheet |

## From another computer's terminal: `gtmux attach`

The phone app watches and drives. From another Mac or Linux terminal you can go
further and work inside a remote session:

```sh
gtmux attach http://<mac>:8765 --token <serve-token> %12   # owner (LAN or tunnel)
gtmux attach 'https://<mac>.example/#g=<token>' %12        # guest (share link)
```

Your local Ghostty / iTerm2 / Terminal becomes the remote tmux pane, fully
interactive, full-screen programs included, over the same connection the phone
uses. `gtmux pair` also prints a one-line `gtmux attach` command that enrolls that
terminal as one of your own devices, so later a bare `gtmux attach <host>` is
enough. A guest is limited to the panes the host allowed it to view and type into
(a view-only pane is read-only), the same scope the web page and the phone
enforce. Set that up in the menu bar's Sharing section or with `gtmux share`:

```sh
gtmux share new --label alice --view %1,%2 --type %1 --expires 24h   # one link with its own scope
gtmux share set <id> --type %2        # change one link
gtmux share revoke <id>               # cut it off
```

Detach with tmux's `<prefix> d` or `Ctrl-]`. Full reference:
[`cli.md` → `gtmux attach`](cli.md) and
[`design/remote-attach-research.md`](design/remote-attach-research.md).

## Security

Everything remote is read-only except typing into a pane, and the pairing token
is the only thing protecting that. With a public tunnel address there is no VPN in
front of it: anyone who has the address and the token can type into your Mac, so
**treat the address plus token like a password**. Don't paste the pairing QR into
a shared channel. A guest link is narrower (only the panes you chose, an optional
expiry, and typing needs `gtmux share on`), and you can revoke it at any time.

When something goes wrong, the phone has kept a record of it: failed requests to the
Mac, each pairing attempt and why it failed, push registration, and the live connection
dropping and coming back. It stays on the phone. Settings → Diagnostics lets you copy or
share it; on the Mac, `gtmux doctor --bundle` packs the other side.

Contributors can read the full protocol in `api/contract.md` in the repo.
