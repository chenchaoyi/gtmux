## Why

A Standard tunnel can retain a valid connector token after its DNS route disappears.
Reinstalling the service currently returns that cached address without repairing it.
Users need a repair command that preserves their device identity and pairing address.

## What Changes

- Add `gtmux tunnel --backend cloudflare --service --force` to repair an existing
  Standard tunnel's ingress and DNS route, refresh its connector token, and reload
  the existing services.
- Require an explicit repair receipt from the Worker so an older deployment cannot
  silently ignore the request. Repair failures leave the local service files intact.
- Do not rotate the device ID, create a replacement tunnel, or overwrite unrelated DNS.
- Explain that network and proxy failures still require a working route to the edge.

## Surfaces

- Terminal: exposes the repair flag and bilingual help.
- Menu bar: uses the repaired Standard route; no new control in this change.
- Phone: keeps the same pairing address; no client changes.
- iPad: shares the same address and pairing behavior as Phone.
- Web: uses the same repaired public route; no UI changes.
