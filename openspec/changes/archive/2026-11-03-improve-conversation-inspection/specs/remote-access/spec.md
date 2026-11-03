## ADDED Requirements

### Requirement: Formatted conversation overview and local bookmarks

The browser conversation overview SHALL render bounded Markdown previews using the message renderer, preserving complete link destinations without loading images or additional conversation content. A user SHALL be able to mark and unmark a turn from its preview. Marked ticks SHALL use the product accent and a distinct dot. Bookmarks SHALL store opaque stable turn identities in an origin/base/credential and home/session scope, survive refresh and history prepending, and clear on authorization revocation. Partial loaded histories SHALL NOT renumber marks. Incoming content and earlier history SHALL preserve the visible reading anchor.

#### Scenario: Earlier turns are loaded around a bookmark

- **WHEN** earlier recorded turns are prepended to a marked conversation
- **THEN** the same original turn remains marked and the visible turn stays in place after overview layout
