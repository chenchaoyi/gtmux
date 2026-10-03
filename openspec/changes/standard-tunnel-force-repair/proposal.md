## Why

A Standard tunnel can retain a valid connector token after its DNS route disappears.
Reinstalling the service currently returns that cached address without repairing it.
Users need one recovery action that chooses repair or replacement for them and preserves
the pairing address whenever the original tunnel remains usable.

## What Changes

- Add `gtmux tunnel --backend cloudflare --service --force` to repair an existing
  Standard tunnel's ingress and DNS route, refresh its connector token, and reload
  the existing services.
- Require an explicit repair receipt from the Worker so an older deployment cannot
  silently ignore the request. Repair failures leave the local service files intact.
- Do not rotate the device ID, create a replacement tunnel, unless recovery confirms the original tunnel is gone; never overwrite unrelated DNS.
- Explain that network and proxy failures still require a working route to the edge.

- Add one **Restore connection** action to the Standard pairing window. It repairs
  in place first, replaces only a confirmed deleted/missing tunnel, and rechecks
  reachability before saying the connection works.
- Add `--service --recover` for that workflow. Provider or network errors never
  trigger a replacement; an address change refreshes the QR and explains re-pairing.
- A connector metric alone does not prove that a phone can reach the address.

## Surfaces

- terminal: exposes the repair flag and bilingual help.
- menubar: one Restore connection action, with honest reachability and re-pairing feedback.
- phone: keeps the same pairing address; no client changes.
- iPad: shares the same address and pairing behavior as the phone.
- Web: uses the same repaired public route; no UI changes.
