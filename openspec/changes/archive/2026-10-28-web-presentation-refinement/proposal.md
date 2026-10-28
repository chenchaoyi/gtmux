# Refine workspace presentation and pairing consistency

## Why
The trace inspector permanently reduces reading width, browser chrome is fixed dark and cramped, browser tabs lack an identifiable icon/context, and the two native pairing entry points present different subsets of the same pairing information.

## What Changes
- Make the trace inspector closeable and reopenable without losing selection or filtering.
- Add persisted System/Light/Dark browser appearance, with terminal captures retaining their original palette.
- Improve workspace branding, sidebar spacing and tab identity.
- Use one shared native pairing delivery component for QR, browser, terminal, code renewal and reachability information.

## Surfaces
- terminal: captures and input contracts stay the same; preserve terminal colors.
- menubar: both native pairing entry points use the shared delivery presentation.
- phone: no client changes; enrollment and share permissions stay the same.
- iPad: no client changes; pairing payloads remain compatible.
- Web: refined trace, appearance, navigation, favicon and contextual tab title.
