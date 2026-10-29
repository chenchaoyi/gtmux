# Native transcript tools

## Why
The browser now separates presentation modes from inspection tools. The phone and
iPad need the same recorded-call reader without adding another mode tab or a new
provider log reader.

## What Changes
- Open Trace from a native session-tools menu, alongside existing Diff when available.
- Share the typed event model and fixtures with the generated browser asset.
- Provide a virtualized searchable record list, session/type filters, explicit follow,
  selectable full details, Copy and existing participant/document previews.
- Preserve the underlying view and draft. Adapt from measured content width; keep
  selected details stable until explicitly refreshed, and retire private reads on
  identity/credential/access changes.

## Surfaces
- terminal: unchanged public transcript contract; no new commands.
- menubar: no new control; browser workspace remains available.
- phone: tools open a full reading surface with list/detail return.
- iPad: the same reader splits list/details only at adequate measured width.
- Web: consume the generated canonical model, retaining delivered tools behavior.

## Impact
- Existing pane/session transcript routes and owner/guest boundaries remain the data source.
- Native session tools and generated browser indexing change; no new endpoint or provider reader.
- Browser compact details and native details temporarily hide filters to increase reading height.
- Physical-device and VoiceOver acceptance are distinct from automated and simulator evidence.
