# Tasks — share-link-is-the-code

## 1. The code belongs to the link
remote-access

- [x] Eight characters, two groups of four, minted with the guest device and persisted
- [x] Redeem is reusable and has no life of its own; the link's expiry and revocation end it
- [x] The one-time machinery (TTL map, spent marks for share codes) goes
- [x] `POST /api/share/code` returns the link's code rather than minting a new one

## 2. One artifact on every surface
remote-access

- [x] `share new` / `share link`: the link with `#code=`, and the two lines to read out
- [x] `gtmux share code` retired
- [x] `gtmux attach` accepts `#code=` in a link, and `--code` as before
- [x] Menu bar: the delivery card shows the short link and the two lines, no mint button
- [x] Phone: the row copies the short link; "read out" shows the two lines

## 3. The browser
browser-mirror

- [x] `#code=` redeems on load, like `#g=`
- [x] The gate's box and its copy drop "once, within ten minutes"
