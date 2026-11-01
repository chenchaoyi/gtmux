# Shared Chat and Trace code reading

Unify the existing code readers, expose static wrapped scripts by language and make expanded call ownership visible. This implements the user's three adjacent Trace feedback items without new server reads or execution.

## Surfaces
- Terminal: Not applicable; captured terminal bytes and input remain unchanged.
- Menu bar: No new Trace reader; recorded source and existing tools remain unchanged.
- iPhone: Shared code cards and nested calls in the existing native reader.
- iPad: The same components adapt to actual pane width and text scale.
- Web: Shared pure parsers/formatters plus matching code cards and nested call structure.

## Boundaries
No evaluation, new authorization, release or Store submission. Full recorded calls and raw modes remain available. Parser bounds fall back to complete original content. Existing home/session, loading, retry and frozen selection remain authoritative.
