# Complete the DSH installation and doctor lifecycle

## Why

DSH public-history reading requires no plugin, but future lifecycle events use the bundled Cordis module and launch overlay. Explicit install writes both correctly. Doctor detects the agent yet routes plugin repairs through the OpenCode-specific migration writer, which can omit the DSH overlay or touch an unrelated legacy OpenCode bridge. Its installed check also accepts a malformed launch overlay. Installation discovery needs a DSH executable probe without enabling deferred TUI radar detection.

## What changes

- Declare plugin writers in the existing installer descriptor and share them between explicit install and doctor repair.
- Validate both owned DSH artifacts, preserve foreign files and profiles, and print an actionable next-launch command without claiming that a running Web process loaded it.
- Detect an installed DSH executable independently of TUI/radar support.
- Keep the existing Claude default and add explicit `--agent auto` selection using detected homes/installations, preserving disabled-hook policy and per-agent writers. Document the distinction between installation and runtime loading.

## Surfaces

- CLI: installation/discovery/doctor and paired help/docs.
- Menu bar: consumes the same CLI diagnostics; no layout or polling change.
- Phone: no API or application change.
- iPad: no API or application change.
- Web: DSH remains running unchanged; only its next intentional launch loads the generated overlay.

## Validation

Use disposable homes and fake executable probes. Reproduce fresh install, existing-module/missing-overlay, stale/foreign overlay, unrelated OpenCode preservation and idempotent repair. No real profile edits, agent restart or model request.
