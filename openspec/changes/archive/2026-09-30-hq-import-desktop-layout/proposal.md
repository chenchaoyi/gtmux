# HQ import desktop layout

## Why
The shipped 800×640 sheet leaves most of archive selection empty, focuses an irrelevant
password field before file selection, and separates primary actions from the footer.

## What Changes
Use compact, content-sized selection/preview and a larger bounded review layout.
Show archive identity and only request a password for an age header. Keep actions in
one footer, provide explicit step context, and retain all backend validation and review gates.

## Surfaces
- terminal: unchanged commands and validation.
- menubar: restore/migration sheets redesigned, bilingual copy and keyboard actions.
- phone: unchanged.
- iPad: unchanged; device acceptance not claimed.
- Web: unchanged.

## Validation
Hosted AppKit renders in both languages and appearances; bounds and conditional-field
regressions, archive header tests, existing import safety tests, Swift suite and make check.
