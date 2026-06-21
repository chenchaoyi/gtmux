## Why

Pairing the phone today means typing the Mac's host + a 32-char token by hand —
error-prone and the worst part of onboarding. The QR schema is already pinned
(`mobileapp/SPEC.md §6`); we just haven't built the two ends: the menu-bar app
that *shows* a pairing QR, and the phone that *scans* it.

## What Changes

- **Menu-bar app (producer):** an "Allow phone access" control that starts
  `gtmux serve` (or surfaces a running one) and renders a pairing QR encoding
  `{v:1, url, token, name}` for the reachable address + serve token.
- **Mobile app (scanner):** a camera-based QR scanner in Pairing that parses +
  validates the QR and pairs in one tap; manual host+token stays as the fallback.

## Capabilities

### Modified Capabilities
- `mobile-app`: add scan-to-pair (camera QR) alongside manual entry.
- `menu-bar-app`: add "Allow phone access" + pairing-QR producer.

## Impact

- Mobile: add `react-native-vision-camera` (camera permission already declared);
  a `ScanScreen`; QR parse reuses `src/pairing/qr.ts`. Camera works on a real
  device only.
- Menu-bar: a Swift QR view (CoreImage `CIQRCodeGenerator`) + a serve lifecycle
  toggle. No third-party deps.
- Read-only invariant unchanged; the QR carries the existing serve token (a
  secret) — shown only on explicit user action, never logged.
- Non-goals: changing the QR schema (v1 is fixed), TLS cert pinning (future),
  the no-VPN remote tunnel (separate increment).
