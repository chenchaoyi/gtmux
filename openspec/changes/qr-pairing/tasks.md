## 1. Mobile QR scanner

- [ ] 1.1 Add `react-native-vision-camera`; `pod install`; confirm simulator build compiles
- [ ] 1.2 `ScanScreen`: camera + QR code scanner, parse via `parsePairingQR`, health+token validate, save to Keychain → Radar
- [ ] 1.3 Wire the Pairing "Scan pairing QR" button to ScanScreen; keep manual entry as fallback; handle camera-permission-denied with a plain message
- [ ] 1.4 Verify on the real device: scan a valid QR → paired; scan junk → clear error

## 2. Menu-bar QR producer

- [ ] 2.1 Swift QR view (CoreImage `CIQRCodeGenerator`) rendering an arbitrary payload string
- [ ] 2.2 "Allow phone access": resolve the reachable URL + serve token, build the `{v:1,url,token,name}` JSON, show it as a QR in the popover (with a privacy note; token shown only on demand)
- [ ] 2.3 Serve lifecycle: detect a running `gtmux serve` or offer to start one; show the reachable host(s)
- [ ] 2.4 `swift build -c release` green; verify the QR view via the accessibility tree / GTMUXBAR_DEBUG

## 3. End-to-end

- [ ] 3.1 Menu bar shows the QR → phone scans it → pairs without typing
- [ ] 3.2 `make check` (Go untouched) + tsc clean; update specs via sync; PR
