# Stable conversation text during refresh

## Why
iPhone conversations can briefly show only the first character of each message. The range renderer starts bounded text at 1px and relies on an asynchronous natural-width event to widen it. Its native view also publishes single-line measurements before a usable constrained width exists and reapplies identical documents.

The follow-up browser screenshot also shows lowercase status section headings while
the App's CollapseBar uppercases category names. The inline status vocabulary is
already shared; category presentation is the missing distinction.

## What Changes
Start bounded text at the available container width, retain the last accepted height and regain available width when short text grows, and do not publish constrained measurements at an unresolved width. Avoid rebuilding identical native documents. Preserve native selection, rich links and unbounded code measurement. Match browser category heading casing to the App while preserving sentence-case Hide/Show actions and lowercase inline statuses.

## Surfaces
- Terminal: no terminal renderer or input changes.
- Menu bar: no renderer changes.
- iPhone: stable first layout and refresh for human and agent prose.
- iPad: the same renderer follows the available width and rotation.
- Browser: App-aligned category heading casing on desktop and mobile; compact layout PR1759 remains delivered independently.

## Verification
Use failing-before/passing-after component regressions for initial width, delayed size events and refresh identity. Extend the standalone synthetic UIKit fixture to reject premature measurements and duplicate relayouts, verify resizing/selection, and inspect phone/iPad rendering. Native fixture and mocked RN checks do not establish full bridge or real-device acceptance.
