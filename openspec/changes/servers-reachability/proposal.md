# The Servers page says which Macs answer and which one is open

## Why

The commander, on the phone (2026-10-05):

- The page should show two layers of information: which servers can be reached, and which
  one is connected. Today only the open Mac has a dot. Every other Mac shows nothing,
  whether it is on or off.
- "Retry sync" is confusing. It sat under a Mac with "Waiting to sync. Notifications may not
  arrive yet." It meant that this Mac's notification setting could not be delivered because
  the Mac could not be reached. Nothing on the page said the Mac could not be reached, and
  retrying cannot make it answer.
- The whole page jumps on every tap. The cause is in the sync, not the layout:
  - Any change to the server list (a bell, a switch, a reorder) set EVERY owner Mac to
    `syncing`.
  - Each row then grew an "Updating…" line, and shrank again as each Mac answered.
  - The open Mac's "Connecting…" line did the same on a switch.
- The radar's way to switch Mac goes unnoticed. The ⇄ square is grey, small, and sits
  between the name and the dot.

## What changes

**Two marks for two facts.**
- A check mark leads the open Mac's name, as in the iOS Wi-Fi list.
- Every row gets a status line whose dot and words say whether that Mac answers:
  - The open Mac shows its live connection: a filled dot, green, amber or red.
  - Every other Mac shows what a probe found: a hollow dot, green "Available", red
    "Can't reach", or grey "Checking…".
- The probe is the unauthenticated `GET /api/health`, sent while the page is shown: on
  arrival, then every 15s, with a short timeout.

**Rows are two lines, always.**
- A pending notification setting is said on the status line ("Can't reach · notification
  setting syncs when it answers").
- A sync in flight is not shown at all.
- So no tap and no probe resizes a row.

**No retry control.**
- When a probe finds a Mac with a pending setting answering again, the app sends the
  setting by itself.
- The warning that it may still notify, or not notify yet, stays on the status line.

**The radar's title is the switch.**
- The title reads `● <name> ⌄`: the connection dot leads, and a brand-coloured chevron
  follows.
- The whole run is the one target into the Servers page. The ⇄ square goes.

## Cost, stated

- **Probe traffic.** While the Servers page is open, one small unauthenticated request per
  Mac every 15s. Nothing is sent when the page is closed.
- **What "Available" claims.** It means the Mac answers, not that it will accept this phone.
  A revoked phone learns that on connecting, where the refusal is already said.

## Not in this change

- Probing Macs from the radar, or in the background. Reachability is shown where you choose
  a Mac, and only while you are choosing.
- Changing what a sync does, or when it runs, beyond the one new trigger: a Mac answering
  again.

## Surfaces

- **终端 / terminal**: not applicable. The CLI talks to one Mac, its own, and has no
  server list.
- **菜单栏 / menubar**: not applicable. The menu bar app runs on the Mac it watches and has
  no list of other Macs.
- **手机 / phone**: this change: the Servers page rows, the probe, the automatic resync,
  and the radar's title.
- **iPad**: identical. The same `ServersScreen` and `RadarPanel`, with the sidebar's
  stacked title keeping the dot and chevron around the name.
- **Web**: not applicable. A shared link opens one pane of one Mac, with no server list and
  no switch.
