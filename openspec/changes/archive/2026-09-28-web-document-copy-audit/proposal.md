# Clarify web access and public documentation

## Why

The browser's invalid-code message assumes every reader received a share link,
although the same form also accepts the Mac owner's pairing code. Other browser
labels expose internal wording such as “waiting” and “fetching”. Public docs
describe the now-typable web view as read-only and reduce remote authorization
to one master token. The browser-mirror spec itself still calls the workbench
view-only despite the later scoped-input requirement.

## What changes

Write role-neutral access and error copy, make browser states describe what a
person can do, and correct the CLI help plus public mobile, knowledge and README
guides in both languages. Keep actual access behavior unchanged.

## Surfaces

- **terminal**: CLI commands are unchanged; `serve` help and security docs are corrected.
- **menubar**: no behavior change; docs describe the existing remote controls.
- **phone**: no behavior change; paired-device and guest-link docs are corrected.
- **iPad**: same documentation as phone.
- **web**: access, permission, loading and empty-state copy is revised.
