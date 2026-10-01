# Mobile connection settings

## Why
The route entry disappears while loading, on request failure, or when only one route is returned. The client conflates failures with a valid empty list; the route page falsely describes that list as one route. Server rows mix connection identity, destructive controls, push preference and pending sync in a dense shared card. The server-mode marker can survive a Mac switch.

## What changes
Keep an owner-only Route entry, show truthful loading/empty/failure/offline states, retain known choices on a failed refresh and discard responses from earlier requests or another Mac. Present each Mac as a separate interactive card with connection state and address separated; put notification sync notices below the switch and removal in a more menu.

## Design
Visual thesis: a calm native utility list with clear identity, restrained status colour and consistent spacing.
Content plan: page title and short orientation; paired Macs with connect and notification controls; guest links; add action.
Interaction thesis: immediate pressed feedback, static loading indicators, explicit sync receipts; no decorative motion under MOBILE's motion rules.

## Surfaces
- terminal: no change; route API contract is unchanged.
- menubar: concise action-first empty state with collapsed, agent-neutral manual instructions.
- phone: persistent owner route entry and readable server cards.
- iPad: the same components in ContentColumn, no independent shell.
- Web: no change; it does not render these mobile settings.

The user also reported a verbose menu-bar empty state with a Claude-only command. This batch folds instructions by default and preserves New session / existing restore actions.

## Verification and limits

- `npm run check -- --runInBand`: TypeScript and ESLint passed (97 existing/style warnings); 1,169 tests passed, 6 skipped.
- `swift test`: 283 tests passed. `swift build -c release` passed. The empty-state test rendered English/Chinese in light/dark, collapsed/expanded at 380pt.
- `GOFLAGS=-p=2 make check` and `bash scripts/check-design.sh` passed, including all 34 capability specs.
- Phone and iPad simulator captures used current production JS with an existing cached native Debug shell and a local fake Mac. Reviewed Servers, Settings and Route layouts; this is not a fresh iOS archive or physical-device acceptance.
- The unsigned cached shell lacks a Keychain entitlement. The visual fixture avoids credential persistence; saving real pairings is covered by existing tests, not claimed as simulator verification here.
- Physical iPhone/iPad reading and layout acceptance remains pending an unlocked device. No release, installation on the user's Mac/phone, ASC upload or Submit was performed.
- Terminal and Web are unchanged. Design docs and local in-app notes are paired in English/Chinese; the previously prepared ASC build 22 does not contain this batch.
