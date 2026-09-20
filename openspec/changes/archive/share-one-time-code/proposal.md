# share-one-time-code — a guest link you can read out loud

## Why

A share link hands over its credential in the URL:

```
https://tunnel.ccy.dev/p35047/#g=1df252867180bc6a5453ea6ba1e5647c14cb86e32f27126666a79d5540ac629e
```

88 characters, 64 of them hex. Where the guest can paste, that is fine. Where they
cannot — a TV browser, a locked-down machine, someone else's laptop that you are reading
your own screen into — it is unusable, which is where this came from: 「token 特别长，在有些
没法直接 copy 的浏览器里，要输入非常困难」 (2026-09-20).

gtmux already solved this problem once, for its OWNER devices, and the two halves never
met. Pairing hands over `#c=<16 hex>`: a **single-use code with a 5-minute life** that the
browser redeems at `POST /api/enroll` for the real token, which it then keeps. A guest
gets `#g=<64 hex>`, which is not a code at all — it IS the token, lasting and revocable,
so it has to be long, so it has to be typed long. `internal/server/web/app.js` states the
asymmetry in its own comment and nobody ever went back to close it.

There is a second reason beyond typing. A lasting credential in a URL is a credential
that gets pasted into a chat, left in a browser history, and photographed off a screen.
A one-time code is worthless the moment it is used.

## What changes

A share link can be handed over as a **short one-time code**, on top of the three doors it
already has (QR, browser link, terminal command).

- The owner mints a code for an existing link. It is bound to THAT link, single-use, and
  lives 10 minutes.
- The guest opens the bare page (`tunnel.ccy.dev/p35047`, 21 characters, no secret in it)
  and types the code. The page redeems it for that link's guest token and keeps it, as it
  already does for an owner pairing code.
- The code is 10 characters of Crockford base32 in two groups (`4F7KQ-9X2TM`): 50 bits,
  no `I`/`L`/`O`/`U`, case-insensitive, so it survives being read out loud and typed by
  someone who is not looking at it.

What does NOT change: the link and its `#g=` token stay exactly as they are (the code is
another door to the same room), the guest's scope and expiry are the link's, and revoking
the link kills everything handed out for it.

Deliberately not in this change, both offered and declined when it was proposed: shortening
the guest token itself (64 hex is more entropy than a revocable link needs, but halving it
still leaves 32 characters to type), and anything new for the QR, which already works.

## Surfaces

- **Terminal (incl. remote attach)** — `gtmux share code <id>` mints and prints the code,
  the bare URL and a QR. `gtmux attach` is NOT given a `--code` this round: an attach guest
  is already typing a command they can paste, and the whole point here is the surface that
  cannot paste. Left to whoever meets it.
- **Menu bar (menubar)** — the share delivery sheet gains a fourth door beside the QR, the link and
  the terminal command: the code, with the same "mint a fresh one" behaviour the pairing
  sheet already has while its window is open.
- **Phone** — the owner manages sharing from the phone (owner-remote-admin), so a link's
  row gains the same code action, reading `POST /api/share/code`.
- **iPad** — the phone's implementation, in the regular shell: no separate work, the row
  is the same row.
- **Web (the browser mirror)** — the surface this change exists for. Its gate screen gains
  a code box; `pair(code)` already posts to `/api/enroll` and stores what comes back.

## Risk and the boundary

The redeem endpoint stays unauthenticated, because the code IS the credential, exactly as
the owner pairing code is. Two things keep that honest: the code is worth 50 bits and lives
10 minutes, and a wrong code answers with the same three reasons pairing already uses
(expired / used / unknown) so a typo is legible without telling an attacker anything they
could not have learned by trying.

A guest code redeems to a token that ALREADY EXISTS. It therefore cannot widen anyone's
scope: it hands over the link's own credential, with the link's own panes and expiry, and
a revoked link's codes die with it.
