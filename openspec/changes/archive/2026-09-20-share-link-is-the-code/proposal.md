# share-link-is-the-code — one artifact, short enough to say

## Why

A share link and its one-time code ended up as two things the owner had to choose
between, and the question that broke it was the commander's: 「我还是没理解两个链接的必要性
到底是什么」 (2026-09-20). The honest answer was that the access behind them is identical,
and the only difference is how long the hand-off stays valid: a link lasts until it is
revoked, a code lasted ten minutes and once.

The code was one-time BECAUSE it was short. Thirty bits cannot stand alone for long. So
the two artifacts existed to buy one property each, and a reader had to hold both in their
head to understand either.

They collapse if the code lasts. At eight characters the arithmetic works: 40 bits against
the redeem limiter's 60 failed tries a minute is one chance in 35,000 per YEAR per live
link, where six characters would be one in 34. So the share link becomes its code:

```
https://tunnel.example.dev/p35047#code=4F7K-Q9X2
```

Forty characters, clickable, pasteable, and readable out loud as two lines. One thing to
hand over, two ways to consume it.

## What changes

A guest link is minted with a CODE beside its token, kept with the link and lasting as
long as it does. The code is the link's public form: the address plus `#code=…`, or the
address and the code said separately.

- Redeeming stays what it was, a trade for the link's own token, which the browser and the
  terminal keep as they already do. It is no longer single-use and no longer expires on its
  own: revoking the link or its expiry ends it, as with everything else about that link.
- `gtmux share code` goes away. It existed to mint the second artifact; there is one now,
  and `share new` and `share link` print it in both forms.
- The raw-token link (`#g=<64 hex>`) keeps working for links already handed out.

The fragment stays a fragment, `#code=` rather than `?code=`: everything before the `#`
reaches the tunnel and lands in someone else's logs, and everything after it never leaves
the browser. The word is the same on both sides, `code` in the URL and `--code` in the
terminal.

## Surfaces

- **Terminal (incl. remote attach)** — `share new` and `share link` print the link and the
  two lines to read out. `gtmux attach` takes the same link or `--code`, and keeps the
  token for that host either way.
- **Menu bar (menubar)** — the delivery card carries the short link, the QR of it, the
  terminal one-liner, and the two lines to read out. No button to mint a second thing.
- **Phone** — the share row copies the short link; "read out a code" shows the two lines of
  that same link.
- **iPad** — the phone's implementation, unchanged by this.
- **Web (the browser mirror)** — reads `#code=` as it already reads `#g=`, and the gate's
  box takes the code typed by hand.

## Risk

Forty bits is a smaller number than the 256 the token carries, and it is the number the
limiter has to hold: failed redeems are bounded per caller and in total, and each refusal
is written to the action log as `act.pair` with its reason, so a sustained run is visible
in `gtmux logs`. A code lives in a URL, as the token did, so it leaks the same ways; what
protects the Mac is that a link is scoped to named panes, revocable in one command, and
free to expire.
