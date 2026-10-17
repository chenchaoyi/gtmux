## Why
Access history currently mixes external client traffic with local menu-bar, CLI and environment-check requests. Credential names and actor groups do not establish a device. The user wants a clear external-access history while retaining complete diagnostics.

## What Changes
- Record server-derived external/local/unknown provenance with a bounded reason vocabulary. Prefer verified client identity; preserve real LAN/tunnel/proxy requests and failures regardless of credential scope or loopback transport.
- Default the shared reader/CLI/API to external records before filtering, pagination and summaries. Keep explicit diagnostic origin ranges and bind cursors/query echoes to the range. Never rewrite old records or fabricate missing identity.
- Keep native and phone/iPad readers on that contract, including old-service detection. Use source labels for non-device remote groups and provide clickable, accessible recording help.
- Document the trustworthy evidence and unknown boundaries in both languages.

## Impact
CLI: new diagnostic origin filter and external default. Menu bar: external history/filter choices/counts and accessible help. Mobile (phone/iPad): same contract and concise help. Web: the existing browser request/audit and HTTP query paths share the server policy; there is no standalone history page today, so no new Web page is added. Remote clients preserve authenticated device identity across transports. No credentials, authorization, retention policy, real history, HQ data, release or installation changes.

## Surfaces
- terminal (including attach): external history by default; explicit diagnostic origin ranges preserve raw audit access.
- menubar: external-only rows, source filters/summaries, and clickable recording rules.
- phone: external-only API contract and accessible rules.
- iPad: the same screen/client and rules as Phone, with existing regular-shell width.
- Web: shared server classification and query API; no standalone access-history page exists, so no new viewer.
