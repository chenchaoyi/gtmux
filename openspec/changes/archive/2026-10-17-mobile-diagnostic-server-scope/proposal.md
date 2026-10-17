# Scope mobile request diagnostics to their server

## Why

The phone record mixes a successful push registration with unrelated HTTP 530/502 failures and omits their server. ApiWatch keys failures only by method/path, so success at a different origin can falsely recover a failing origin or suppress its failure. Historical records have no recoverable source.

## What Changes

- Key request failure sampling and recovery by sanitized origin plus method/path.
- Retain and display the origin on request failures/recovery and the sanitized base address on push-registration receipts.
- Preserve failure severity, bounded storage, route prefixes and legacy records without inventing their source. No request, push preference or retry behavior changes.

## Surfaces

- terminal CLI: unchanged; no core or transport changes.
- menubar: unchanged; its diagnostic store is separate.
- phone and iPad: shared request diagnostics and localized descriptions change.
- Web: unchanged; no mobile diagnostics consumer.

## Validation

Synthetic tests reproduce cross-origin suppression/false recovery, distinguish base-path routing, check credentials/query/fragment redaction and exercise the real client-to-buffer-to-display path. Run mobile and repository gates.
