# Share browser entry under a tunnel path

## Why

The generated guest URL ends at `/p<port>#code=...`. Its embedded HTML used relative assets, which a browser resolves at the origin root when the path has no trailing slash. The inspected live path returned the full HTML, but `/app.js` and `/style.css` returned 404 while `/p<port>/app.js` and `/p<port>/style.css` returned 200. The screenshot shows the unstyled gtmux header, before code redemption can run.

## Changes

Establish a same-origin document base before the first asset reference, activate resource attributes only after the base is set to prevent speculative cross-prefix loads, derive the API prefix from that same bootstrap, and accept root, slash/no-slash tenant entries and explicit index.html without moving credential fragments into HTTP requests. Retain existing share URLs and access policy; no tunnel configuration or credential mutation is needed.

## Surfaces

- terminal (CLI): URL format and share scope unchanged.
- menubar (Mac): generated sharing URLs unchanged; recipients benefit from the repaired browser entry.
- phone (iPhone): share URLs unchanged; no native source changes.
- iPad: same as Phone; physical layout is outside this repair.
- Web: bootstrap before assets; APIs, fonts and vendor resources stay in the selected tenant path.

## Validation

Real-browser before/after reproduction with synthetic codes and API fixtures, desktop + phone widths, root and prefixed paths, automatic redemption + manual code retry, console/network assertions, node regression tests, make check and design/spec gate. Tesla's embedded browser cannot be remotely exercised and will remain an explicit limit.
