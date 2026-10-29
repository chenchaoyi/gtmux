# Remote access reference

**English** · [中文](phone.zh.md)

Connection methods, device permissions and notification requirements.

**Connecting an iPhone, iPad or browser for the first time?** Follow
[Use your phone, iPad and browser](guides/phone-and-web.md) for setup, daily use and sharing.

## Server platforms

Pair with a macOS or Linux host running gtmux. The server list, routes, sharing, terminal/chat and HQ use that host's state. Linux does not offer desktop window focus or lid controls. [Linux setup](linux.md).

## Connection methods

The live radar and terminal need a reachable Mac running `gtmux serve`. The Mac opens
outbound tunnels, so no inbound port forwarding is needed.

| Method | When to use it | Address and requirements |
|---|---|---|
| Local network | Devices can reach each other on the same network | Mac address, normally port 8765 |
| Standard tunnel | Access from other networks | Free, stable `https://gtmux-<label>.ccy.dev`; uses `cloudflared` on the Mac |
| Direct tunnel | The network blocks Standard | Paid access code; port 443 through gtmux's server; no `cloudflared` needed |
| Quick tunnel | A temporary connection | `trycloudflare.com` address changes on every run; devices must pair again |
| Tailscale or another VPN | You already connect your devices through a VPN | Reachable VPN address, normally port 8765 |

A tunnel still needs both networks to allow the connection. Corporate or guest Wi-Fi
may isolate devices even when they use the same Wi-Fi name.

### Tunnel settings

`gtmux tunnel` defaults to Standard and reuses an active always-on tunnel.
`--service` keeps it running across reboots; `--unservice` removes the service;
`--status` shows the active method and address. If Direct is already active,
`--service` keeps Direct. Use `--backend cloudflare` explicitly to switch to Standard.

With an updated gtmux, always-on Standard access recovers automatically when a tunnel
is deleted or reclaimed after a long absence. Your saved address and phone/iPad pairings
stay the same. To request recovery now: `gtmux tunnel --backend cloudflare --service --recover --yes`.

In the Mac Preferences or Pair your phone window, a failed route lookup offers **Retry**. Reopening either window refreshes the list when Direct is active. If a refresh fails, the previous routes and measurement time remain visible; a list error alone does not mean the active tunnel has disconnected.

Direct is unlocked at [gtmux Direct](https://ccy.dev/projects/gtmux/direct), then redeemed
with `gtmux tunnel --redeem <code>`. One code covers up to three Macs; each gets its own
address and account. `--servers` lists routes and measured latency; `--server <id>` moves
a Mac while keeping its code and port. Devices that have connected before follow the
move; devices that only paired must scan again. Existing guest links stop working.

Full flags: [`gtmux tunnel`](cli.md#gtmux-tunnel).
Self-hosting: [the tunnel design](design/remote-access-tunnel.md) and `deploy/self-tunnel/`.

## Pairing and guest permissions

Pairing gives each of your devices an owner credential. It can read and type into panes,
manage guest links and create new pairing codes. Treat that credential as a password.

A pairing code works once and expires in five minutes. Generate a fresh code for each
device. If the address changes before enrollment, refresh the code. After successful
enrollment, a failed first radar load only needs a connection retry.

Guest links grant access to specified panes. `--view` permits reading; `--type` permits
input and also adds the pane to view. Input additionally requires the master switch
`gtmux share on`. `--expires` accepts values such as `45m`, `24h` or `7d`; without it,
the link does not expire. Revoking a link refuses its next request.

Guests cannot access the HQ page, owner settings, push registration, session creation or
`gtmux attach`. A guest who can type uses the permissions of the program in that pane:
pane scope does not sandbox its filesystem or network access.

Revoke a lost device on the Mac with `gtmux devices revoke <id>`. The phone's UI does
not revoke owner devices or switch remote access on and off; those limits do not
restrict what its owner credential can do. Never publish pairing codes, device
credentials or guest links. A public tunnel address alone does not grant access.

Full flags: [`gtmux pair`](cli.md#gtmux-pair-enroll-your-own-devices-full-control),
[`gtmux share`](cli.md#gtmux-share-scoped-revocable-access-for-a-collaborator) and
[`gtmux devices`](cli.md).

## Message delivery

After sending, the temporary chat bubble distinguishes Sending from Sent to terminal. The latter confirms terminal delivery, not that the agent has read or finished it. The bubble is removed when a new matching prompt reaches the transcript, even if later turns arrive in the same refresh; it does not reappear as the conversation advances.

## Availability and notifications

The Mac must remain awake for remote access. A closed MacBook normally sleeps;
`gtmux awake on` keeps it running with the lid shut, but does not start serve or the
tunnel. On battery, it warns at 30% and restores sleep at 20%.

Notifications do not need a direct phone-to-Mac connection. They need an awake Mac
running serve with access to its push relay, a registered iOS device and Apple's push
service. Notification settings and network conditions can delay or block delivery.
The live radar and replies still need a connection to the Mac.

Agents outside tmux are read-only: without a pane, there is nowhere to send input.

## Usage widgets

iOS/iPadOS 17+ supports Home Screen and Lock Screen usage widgets. They follow the
current owner server and show provider quota or balances with reading times. See
[usage widgets](widgets.md) for setup, refresh limits, privacy and macOS scope.

## HQ shortcuts

On the phone radar, hold the floating HQ button and choose **Usage** or **Knowledge base**.
A tap opens HQ; dragging moves the button. Shortcuts are available when HQ is running;
if it has not started, the button explains how to start it on the Mac. Compact iPad windows
use the same button; the iPad sidebar keeps the resource entrances on the HQ page.

## Desktop conversations

ChatGPT desktop Codex conversations appear in **Desktop apps**, initially marked
**Status only**. They do not affect managed totals, HQ attention, push notifications
or Live Activity, and their conversation content is excluded from digest and mining.

On a paired phone or iPad, tap a conversation to read its live-updating Chat.
Viewing it does not enable HQ follow. Choose **Follow settings** from its long-press
menu to allow HQ reading, analysis and progress reports. Switches save automatically;
use **Done** to close. **Conversation notifications** and **Save to knowledge base** are
independent choices, initially off and unavailable until HQ follow is on. Knowledge
capture covers future activity, not earlier messages.

The list badge updates only after the Mac confirms the current settings. A conflict
or missing save receipt triggers a fresh read; if that fails, use Reload before making
more changes. Turning off **HQ follow** also clears both optional permissions and keeps
existing records. Re-enabling follow does not restore the optional permissions.
Settings apply to this conversation on this Mac, not every desktop conversation or
another Mac. Continue conversations in ChatGPT desktop: these settings do not enable
terminal input or approval from gtmux. An older Mac core needs updating to expose
settings. Guests have no desktop follow controls.

## All panes

In **All panes**, the vertical expand/compress button matches the Mac pane browser.
Tap the magnifying glass to focus search; session names, commands, directories and
pane/window IDs are searchable.

## Troubleshooting

- **Cannot connect on the same Wi-Fi:** test `http://<mac-ip>:8765/api/health` from the
  device. If it cannot load, check the network path or use a tunnel/VPN.
- **A used pairing code is refused:** generate a new code; each enrolls one device.
- **A guest can see but cannot type:** check the link's input panes and `gtmux share on`.
- **“Share access has expired”:** the pane grants belong to a previous tmux server. Ask the owner to grant access to the intended panes again. Web, phone and iPad keep the guest connection and show this explanation; pairing again does not restore these grants. An ordinary empty shared radar does not mean access has expired.
- **No notifications:** check the Mac is awake, serve is running, the device is registered
  and iOS allows notifications.

The app's **Settings → Diagnostics** exports its local connection and pairing log.
`gtmux doctor --bundle` collects the Mac side. Protocol details: [API contract](../api/contract.md).

HTTP 304 means the conversation has not changed; the app keeps its cached content.
It is not a connection failure. Older 304 warnings remain in raw exports but appear
as **Content unchanged**, outside the problems filter. Leaving a page cancels its read
without adding a network warning. App-imposed deadlines are recorded as timeouts;
other **Request did not complete** entries mean no response arrived, not that the whole
Mac was offline. Compare their timestamps with later successful reads and the Mac's bundle.

## Review server access
Open Settings → Manage server → Access history to inspect device/browser requests and HTTP results in a compact log. Local times include seconds, dates follow the app language, and full identity/source metadata is visible without expanding rows. This is owner-only; guest links cannot read it. Refresh starts at the newest records; Load earlier keeps the current list. An old server must be updated before it supports history. Details about sampling, private storage and permission limits are in [Server access history](design/server-access-audit.md).

In Access history, choose Logs or Devices. Filter by a full pairing ID, operation or outcome; selecting a device summary opens that device's log. Clear broadens the view. Continue summary reads earlier pages when the total is still partial. Counts describe recorded activity: successful reads are sampled, and device names do not identify a unique pairing. Older servers must be updated for filtering/grouping.

For the Standard client version floor, update channel and connector restart, see [security updates](install.md#standard-tunnel-security-updates).

## Trace records

Open a session’s wrench menu and choose **Trace** to inspect loaded messages and recorded
tool calls. Search and type/session filters cover the loaded history. On iPhone, select
a record and close its details to return to the same list; a wide iPad keeps both columns
visible. Closing details expands the list. Copy uses the original text; new content waits
for **Update details**. Use **Follow latest** to return to current activity. Closing Trace
keeps your conversation or terminal and its unsent draft. The same tool is available in
HQ and readable external conversations; external sessions stay read-only.

Trace shows recorded inputs and summaries. It cannot supply absent tool outputs, model
request timing or token usage. The disclosure **About these records** explains this scope.
