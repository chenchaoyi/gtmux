# Web-shared input — host-consented, per-pane typing on the shared web page

## Why

`gtmux serve`'s web mirror is deliberately READ-ONLY ("the browser mirror can't send;
reply on phone/Mac"). Input today is the owner's own: the mobile app / local terminal
hit `POST /api/send`, gated only by a valid Bearer token (master or a paired device).

The user wants a collaborator on the SHARED web page to be able to TYPE into the
terminal — but safely: input into a terminal is command execution, so it must be
(1) explicitly CONSENTED by the host, and (2) limited to the panes the host CHOOSES.

## The core tension (security)

A shared link is a URL + a credential. If the shared credential is the owner's own
token, a per-pane allowlist is meaningless — the holder can just call `POST /api/send`
directly and type anywhere. So enabling shared input REQUIRES a credential the owner
can hand out that is INPUT-RESTRICTED by construction, distinct from the owner's full
credential.

gtmux already has the pieces: the enroll roster stores per-token credentials, each
individually revocable (a device authenticates with its OWN token, never the master).
We extend that with a token SCOPE.

## What Changes

- **Token scope.** Each credential is `full` (the owner — the master token and paired
  devices) or `guest` (a share link). `auth()` resolves the scope and carries it in
  the request context.
- **Consent, default OFF.** A serve-side `shareInput` flag (persisted). While off, a
  guest gets the read-only mirror — no input, today's behavior. The host turns it on
  explicitly.
- **Per-pane allowlist.** The host selects which pane ids guests may type into. A
  guest's `POST /api/send` is allowed ONLY when consent is ON AND the pane is on the
  allowlist; otherwise 403. A `full` token is unaffected (the owner keeps full input).
- **Guest share links.** `gtmux share` mints a guest token + share URL, lists and
  revokes them, toggles consent, and edits the allowlist. Each guest link is revocable
  on its own, so "stop sharing with X" is one command.
- **`GET /api/share`** returns the CALLER's input capability (is input on, which panes)
  so the web page shows an input affordance only where the viewer is actually allowed —
  UI reflects the server gate, never the reverse.
- **Web mirror gains input.** For allowed panes only, the browser mirror lets the guest
  type; typing goes through the same `POST /api/send`, which enforces the gate.

## Non-goals / invariants

- The server gate is authoritative; the web UI only mirrors it. A guest hitting
  `/api/send` for a non-allowlisted pane is refused regardless of the UI.
- Default is OFF and no panes — sharing input is strictly opt-in, per pane.
- Master + device tokens keep full input (no regression to the owner's own control).

## Impact

- Specs: `remote-access` (token scope + `/api/send` gate + `gtmux share`),
  `browser-mirror` (web input for allowed panes + `/api/share`).
- Code: `internal/server` (auth scope context, handleSend gate, share manager +
  `/api/share`, guest mint/revoke), `internal/app/serve.go` (wire + persist share
  state), a `gtmux share` command, `internal/server/web/app.js` (input UI).

## Open forks

See `design.md` §Forks — the credential model and the v1 control surface — surfaced to
the host before implementation (security-sensitive).
