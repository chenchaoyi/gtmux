# remote-access Specification

## ADDED Requirements

### Requirement: Guest tokens with consented, per-pane shared input

The system SHALL let the host share the web page such that a collaborator can type into
the terminal ONLY with the host's explicit consent and ONLY into panes the host selects,
without ever granting that collaborator the host's own full control. Every credential
SHALL carry a SCOPE: `full` (the master token and paired devices — the owner's own
surfaces, unrestricted input) or `guest` (a share link). `POST /api/send` SHALL enforce
the scope: a `full` caller is unrestricted; a `guest` caller SHALL be allowed only when
shared input is CONSENTED (a host toggle, default OFF) AND the target pane is on the
host's per-pane allowlist, else `403`. The gate is server-side and authoritative — the
web UI only mirrors it.

The host SHALL control this via `gtmux share` (status; on/off consent; add/remove
allowlist panes; mint a guest share link with a URL + QR; revoke a guest link
individually). Guest tokens SHALL live in the same revocable roster as devices
(persisted), so revoking one share stops exactly that link. `GET /api/share` SHALL
return the CALLER's input capability (`{input, panes}`) so a surface can show input only
where allowed. The default SHALL be no consent and no panes — shared input is strictly
opt-in, per pane. (A menu-bar control surface mirroring `gtmux share` is a planned
follow-up, not required by this capability.)

#### Scenario: A guest is blocked until consent AND allowlist

- **WHEN** a `guest` token `POST`s `/api/send` for a pane while consent is off, or for a
  pane not on the allowlist
- **THEN** the send is refused (`403`) and the terminal is not touched

#### Scenario: A guest types into an allowed pane

- **WHEN** consent is on and the pane is on the allowlist, and a `guest` token sends to it
- **THEN** the input is delivered, the same as a full caller would

#### Scenario: The owner keeps full input

- **WHEN** the master token or a paired device sends to any pane
- **THEN** it is unrestricted, regardless of the consent toggle or allowlist

#### Scenario: A share link is revoked on its own

- **WHEN** the host revokes one guest share link
- **THEN** exactly that link's token stops working; other guests and the owner's own
  devices are unaffected
