## MODIFIED Requirements

### Requirement: Guest tokens with consented, per-pane shared input

The system SHALL let the host share the web page such that a collaborator SEES only the
panes the host chose to show and can type into the terminal ONLY with the host's explicit
consent and ONLY into panes the host selects, without ever granting that collaborator the
host's own full control. Every credential SHALL carry a SCOPE: `full` (the master token
and paired devices — the owner's own surfaces, unrestricted view and input) or `guest` (a
share link).

The host SHALL keep TWO independent per-pane allowlists for `guest` callers: a **view**
allowlist (which panes a guest may SEE) and an **input** allowlist (which panes a guest
may TYPE into). The invariant **input ⊆ view** SHALL hold: allowing input on a pane SHALL
imply view on that pane, and removing view from a pane SHALL remove its input. `POST
/api/send` SHALL enforce the scope: a `full` caller is unrestricted; a `guest` caller
SHALL be allowed only when shared input is CONSENTED (a host toggle, default OFF) AND the
target pane is on the input allowlist, else `403`. Both gates are server-side and
authoritative — the web UI only mirrors them.

The default SHALL be secure: a fresh guest sees NO panes and may type into NONE — consent
off, empty input allowlist, and empty view allowlist. Shared view and shared input are
each strictly opt-in, per pane.

The host SHALL control this via `gtmux share` (status; on/off consent; add/remove input
panes; `share view on/off` and `share view add/remove` for the view allowlist; mint a
guest share link with a URL + QR; revoke a guest link individually). `gtmux share add %N`
(input) SHALL imply view on that pane. Guest tokens SHALL live in the same revocable
roster as devices (persisted), so revoking one share stops exactly that link. `GET
/api/share` SHALL return the CALLER's capability (`{input, view, panes, viewPanes}`) so a
surface can show each pane only where allowed. `gtmux share status`/`new` SHALL each
support additive `--json`: `status --json` returns `{enabled, panes, view_panes,
guests:[{id, label, enrolled_at}], base}` and `new --json` returns `{id, label, url}`,
carrying NO bare token.

#### Scenario: A guest is blocked until consent AND input allowlist

- **WHEN** a `guest` token `POST`s `/api/send` for a pane while consent is off, or for a
  pane not on the input allowlist
- **THEN** the send is refused (`403`) and the terminal is not touched

#### Scenario: A guest types into an allowed pane

- **WHEN** consent is on and the pane is on the input allowlist, and a `guest` token sends
  to it
- **THEN** the input is delivered, the same as a full caller would

#### Scenario: Allowing input implies view; removing view removes input

- **WHEN** the host adds a pane to the input allowlist, then later removes that pane from
  the view allowlist
- **THEN** adding input marks the pane viewable, and removing view drops the pane from the
  input allowlist too, so a guest can never type into a pane it cannot see

#### Scenario: The owner keeps full view and input

- **WHEN** the master token or a paired device reads or sends to any pane
- **THEN** it is unrestricted, regardless of the consent toggle or either allowlist

#### Scenario: A share link is revoked on its own

- **WHEN** the host revokes one guest share link
- **THEN** exactly that link's token stops working; other guests and the owner's own
  devices are unaffected

#### Scenario: The `--json` contract carries no token

- **WHEN** a consumer runs `gtmux share status --json` or `gtmux share new --json`
- **THEN** the output is machine-readable and includes both allowlists (`panes`,
  `view_panes`) and the guest list / minted URL but no bare token field

## ADDED Requirements

### Requirement: Guest read access is scoped to the view allowlist

For a `guest`-scope caller, the server SHALL filter every read surface to the guest's
view allowlist: `GET /api/agents` SHALL return only the rows for viewable panes, `GET
/api/pane` SHALL refuse (`403`) a pane that is not viewable, and the SSE agent stream and
the web terminal mirror SHALL likewise expose only viewable panes. `full`-scope callers
(master token, paired devices) SHALL be unfiltered. The filter is server-side and
authoritative.

#### Scenario: A fresh guest sees nothing

- **WHEN** a guest opens a newly minted share link whose view allowlist is empty
- **THEN** `GET /api/agents` returns an empty radar, `GET /api/pane` refuses every pane,
  and the web page shows no sessions to view

#### Scenario: A guest sees only allowed panes

- **WHEN** the host adds pane `%A` to the view allowlist and a guest reads `/api/agents`
  and `/api/pane`
- **THEN** the guest's radar contains only `%A`'s row and `/api/pane` returns text for
  `%A` but refuses any other pane (`403`)

#### Scenario: The owner's read is unfiltered

- **WHEN** the master token or a paired device reads `/api/agents` or `/api/pane`
- **THEN** the full radar and any pane's text are returned, unaffected by any guest view
  allowlist
