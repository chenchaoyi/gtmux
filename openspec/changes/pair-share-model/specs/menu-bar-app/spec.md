# menu-bar-app — delta

## ADDED Requirements

### Requirement: Preferences present the two-track pair/share model

The Preferences window SHALL organize remote capability into the two-track model:
a 远程访问/Remote-access section (the door: Off / Wi-Fi / Anywhere, unchanged), a
你的设备/Pair section, and a 分享/Share section — so "my own surfaces" and
"collaborator access" never mix.

The Pair section SHALL list paired (owner-scope) devices — name, a kind icon,
last-seen, and per-row revoke — plus a single "配对新设备/Pair a device" action
opening one sheet that renders the SAME enroll code in the three media (phone QR /
browser URL+code / terminal attach one-liner).

The Share section SHALL carry the consent master switch and the guest-link list —
each row showing the label, a scope summary (viewable count · typable count ·
expiry if any), created-at, and revoke — with a per-link inline scope editor (the
See/Type per-session columns) and a "新建分享/New share" sheet that names the link
AND selects its sessions in one step. Editing a link's scope SHALL affect ONLY
that link (the legacy global broadcast forms are not used by this UI).

#### Scenario: Pair and Share never mix

- **WHEN** the user opens Preferences with two paired devices and two share links
- **THEN** the devices appear only under Pair and the links only under Share, each
  with its own list styling and actions

#### Scenario: A share is created with its scope in one step

- **WHEN** the user clicks 新建分享, names it "Alice", ticks session A as
  See+Type, and confirms
- **THEN** one link is minted whose scope is exactly that selection, the URL is
  copied/surfaced, and other links' scopes are untouched

#### Scenario: Per-link editing touches one link

- **WHEN** the user expands link "Alice" and unticks a session's Type
- **THEN** only Alice's input allowlist changes; other links and the template are
  unaffected
