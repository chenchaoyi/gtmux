# Batch photo selection and upload feedback

Allow Photo Library multi-selection in the existing shared Composer. One photo opens the editor; multiple photos stage together and remain individually editable. Replace thumbnail percentage overlays with compact measured progress and preparation/saving phases. Keep drafts, ordered uploads, input authorization and explicit send semantics.

## Surfaces

- Terminal: no selection or HTTP protocol change.
- Menu bar: no change; the native screenshot editor is separate.
- Phone: system multi-picker, staged edit and upload feedback in shared Composer.
- iPad: the same Composer and native picker in compact/regular layouts.
- Web: existing browser selection remains unchanged; no fabricated upload completion is introduced.

## Verification boundaries

Synthetic production-component and XHR tests, unchanged existing single-photo/input-fence suites, TypeScript/lint/full App gate and source conformance. No device, live send, install or release claim.
