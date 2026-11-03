## ADDED Requirements

### Requirement: Formatted conversation overview and local bookmarks

The browser conversation overview SHALL render bounded Markdown previews using the message renderer, preserving complete link destinations without loading images or additional conversation content. A user SHALL be able to mark and unmark a turn from its preview. Marked ticks SHALL use the product accent and a distinct dot. Bookmarks SHALL store opaque stable turn identities in an origin/base/credential and home/session scope, survive refresh and history prepending, and clear on authorization revocation. Partial loaded histories SHALL NOT renumber marks. Incoming content and earlier history SHALL preserve the visible reading anchor.

#### Scenario: Earlier turns are loaded around a bookmark

- **WHEN** earlier recorded turns are prepended to a marked conversation
- **THEN** the same original turn remains marked and the visible turn stays in place after overview layout

#### Scenario: Recent-history window rolls while reading

- **WHEN** a new turn pushes an older loaded turn outside the bounded history window
- **THEN** a still-loaded visible turn keeps its screen position
- **AND** tool disclosures and code-source selectors follow their real conversation identity rather than the changed list offset
