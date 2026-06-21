## ADDED Requirements

### Requirement: Scan to pair

The system SHALL let the user pair by scanning the menu-bar app's pairing QR
(camera), parsing and validating the `{v:1,url,token,name}` payload, then pairing
without manual entry. Manual host+token SHALL remain available as a fallback.

#### Scenario: Scan a valid pairing QR

- **WHEN** the user taps "Scan pairing QR" and points the camera at a valid v1
  pairing code
- **THEN** the app parses url+token+name, verifies reachability + the token, saves
  the pair to the Keychain, and shows the Radar

#### Scenario: Invalid or unsupported code

- **WHEN** the scanned code is not a v1 gtmux pairing payload
- **THEN** the app rejects it with a clear message and stays on the scanner

#### Scenario: Camera permission denied

- **WHEN** camera access is denied
- **THEN** the app explains it plainly and offers the manual host+token fallback
