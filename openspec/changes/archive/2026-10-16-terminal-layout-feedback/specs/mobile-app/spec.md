## ADDED Requirements

### Requirement: Terminal layout transitions preserve reading context
The phone and iPad terminal SHALL show shared branded loading feedback before rebuilding for a Wrap/Original change. Scrollers SHALL remain mounted. A reader in history SHALL remain at the same captured line and character area as far as the target layout permits, including partial-row offset and horizontal position. Readers following live output SHALL remain at the tail. Intermediate native scroll/size callbacks SHALL NOT change that intent.

Restoration SHALL complete when content height is unchanged. Superseded requests and unmounted views SHALL cancel scheduled work. Active native selection SHALL defer layout changes. Loading SHALL finish after applying the latest layout and restoring position, then resume the existing snapshot policy.

#### Scenario: A reader switches while viewing a long line
- **WHEN** a reader switches Wrap to Original and back
- **THEN** feedback appears before reflow and the same captured line/character remains the reading anchor
- **AND** intermediate clamping does not jump the reader to live output

#### Scenario: The layout does not change content height
- **WHEN** a layout switch keeps the same row count
- **THEN** restoration and loading completion do not depend on a new content-size event

#### Scenario: A request is superseded
- **WHEN** another layout is requested before the previous one settles or the view unmounts
- **THEN** obsolete scheduled callbacks do not apply a stale layout or position
