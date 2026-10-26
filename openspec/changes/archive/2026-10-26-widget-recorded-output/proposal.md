# Recorded output in usage widgets

## Why

The local DSH account adapter currently has no balance observation. A synthetic
balance proved formatting only, but the demonstration suggested a working account
integration. The core already reports daily and seven-day recorded output.

## What Changes

Use those output counts for providers without quota or balance readings. Keep the
units, source day and scan time explicit; absent history remains unavailable.
Update the synthetic fixture and user documentation to demonstrate recorded output.

## Surfaces

- terminal: existing usage history contract is unchanged.
- menubar: existing usage display is unchanged; shared widget views gain output.
- phone: small, medium and Lock Screen widgets show a typed output fallback.
- iPad: the same widget projection and layouts as Phone.
- Web: existing usage display is unchanged.

## Impact

Display-only optional snapshot field; no new API, credentials, permissions or polling.
No native Mac extension, release, installation or Store submission in this change.
