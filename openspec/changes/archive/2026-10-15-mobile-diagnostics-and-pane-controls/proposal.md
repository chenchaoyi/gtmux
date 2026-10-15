# Honest mobile request diagnostics and recognisable pane controls

## Why
The transcript consumer correctly keeps cached content on HTTP 304, but its generic
fetch observer records that response as a warning. Expected lifecycle aborts are also
reported as network failures, while timed aborts lack a reliable timeout reason. The
pane browser's opposed chevrons look like a diamond and its Unicode search glyph is
smaller than the surrounding controls.

## What Changes
Treat GET/HEAD 304 as successful validation. Ignore explicitly cancelled requests in
problem diagnostics while retaining marked deadlines and real HTTP/network failures.
Present historical false 304 warnings as unchanged content without rewriting exports.
Use evidence-based request wording, not an elapsed-time guess that the Mac is offline.
Match the Mac pane browser's vertical expand/compress metaphor and replace the search
character with a legible vector magnifier and 44-point focus target.

## Surfaces
- terminal: no changes.
- menubar: no changes; its pane-browser icons are the reference.
- phone: diagnostic classification/wording and pane-browser controls above.
- iPad: same shared mobile components and request handling.
- Web: no changes.

## Non-goals
No tunnel reconfiguration, new retry behavior, automatic mutations, log deletion,
release or installation. The screenshot alone cannot identify a physical network cause.
