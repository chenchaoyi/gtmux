# mobile-app — delta

## ADDED Requirements

### Requirement: The app separates paired Macs from guest connections

The app's server list SHALL present the two-track model: paired Macs (owner
scope) under "我的 Mac/My Macs" and share-link connections (guest scope) under
"访客连接/Guest access", never intermixed. A guest connection SHALL display its
granted scope (how many sessions are viewable and how many typable, from
`GET /api/share`), and guest-mode copy SHALL say it is connected via a share
link (分享) rather than paired (配对).

#### Scenario: The list reads the two tracks

- **WHEN** the user has one paired Mac and one share-link connection saved
- **THEN** the server list shows the Mac under 我的 Mac and the guest connection
  under 访客连接, the latter labelled with its granted scope

#### Scenario: A guest connection shows its access

- **WHEN** the app is connected over a share link that grants 2 viewable / 1
  typable sessions
- **THEN** the guest banner/scope line reads that count, sourced from the
  caller-scope endpoint
