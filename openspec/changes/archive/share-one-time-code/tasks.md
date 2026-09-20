# Tasks — share-one-time-code

## 1. The code itself, and redeeming it
remote-access

- [x] Crockford base32 minting + normalization (`I`/`L`→`1`, `O`→`0`, case, dashes)
- [x] `EnrollManager.MintShareCode(deviceID)`: bound to an existing guest device, single
      use, 10-minute life, refused for a device that is not a guest or has expired
- [x] `RedeemWhy` answers a guest code with that device, creating nothing, and keeps the
      three existing reasons for a code it will not take
- [x] `POST /api/share/code` (owner auth) mints; `POST /api/enroll` redeems both kinds
- [x] The act trail says which kind was redeemed (`act.pair` with the link's scope)

## 2. The surfaces that hand it over
remote-access

- [x] `gtmux share code <id>`: the code, the bare URL, a QR
- [x] Menu bar: the fourth door in the share delivery sheet
- [x] Phone: the same action on a share link's row

## 3. The surface that takes it
browser-mirror

- [x] The gate screen's code box, in both languages, with the refusal reason shown
- [x] A redeemed code leaves no secret in the URL or in history
