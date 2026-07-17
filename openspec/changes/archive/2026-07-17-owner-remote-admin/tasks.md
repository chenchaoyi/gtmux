# Tasks: owner-remote-admin

## 1. Server — authorization (decision B) + the guest-leak fix

- [x] 1.1 Add `fullOnly(w,r) bool` (scope != guest → true, else 403 host-only-ish
      message). Swap `masterOnly → fullOnly` on handleShareConfig / handleShareNew /
      handleShareSet; guard handleDevices with fullOnly (currently unguarded).
- [x] 1.2 Scoped revoke: `EnrollManager.RevokeBy(id, allowDevice bool)` — master
      path allowDevice=true; owner path allowDevice=false (refuse a non-guest
      target). handleRevoke: guest → 403; owner device → RevokeBy(id,false) with a
      403 "paired devices are managed on the Mac" when the target is a device;
      master → RevokeBy(id,true).
- [x] 1.3 Tests: owner can config/new/set/list + revoke a guest link; owner CANNOT
      revoke a device (403); guest refused on devices/list, revoke, and all share
      management; master unchanged. Regression pin for the closed leak.
- [x] 1.4 api/contract.md: authorization notes on /api/devices, /api/devices/revoke,
      /api/share/{config,new,set}.

## 2. Mobile — the owner management screen

- [x] 2.1 client.ts: `devices()`, `shareConfig()`, `setShareEnabled(...)`,
      `shareNew(label,view,input)`, `shareSet(id,view,input)`, `revokeShare(id)`,
      `shareLink(id)` (typed wrappers over the endpoints). Jest for request shape.
- [x] 2.2 ManageMacScreen (owner-only): consent toggle · per-link See/Type editor ·
      create-share (Alert.prompt name → mint with the global template) · copy-link ·
      revoke-link · read-only device roster + the Mac-only note. i18n (en+zh). Entry
      gated on `!isGuest` (from AgentsContext), in Settings › Connection.
- [x] 2.3 Jest: `nextLinkScope` pins the Type ⊆ See invariant the editor enforces;
      client.test pins each method's verb/path/body + the guest/device roster split.
      (The `!isGuest` gate is a one-line conditional, verified by inspection.)

## 3. Consistency + verification

- [x] 3.1 Fold spec deltas (remote-access, mobile-app); openspec --strict green;
      archive change. (Also added a `GET /api/share/link` re-view requirement + the
      `gtmux share link` CLI, documented in cli.md.)
- [x] 3.2 make check + CGO_ENABLED=0 + mobile npm run check green.
- [ ] 3.3 Dogfood: from the phone (owner), create a scoped link, edit its See/Type,
      revoke it; confirm a guest connection shows no management entry and the
      endpoints 403. (Pending the device build in the release step.)
