## ADDED Requirements

### Requirement: HQ overview makes withheld matters discoverable
Phone and iPad SHALL use the same overview disclosure component to expose the
visible/total and remaining matter count, with a clear expand/collapse control and
accessible expanded state. The presentation SHALL preserve canonical matter state.

#### Scenario: An overview contains more matters than its preview
- **WHEN** only three of twenty matters are shown
- **THEN** the remaining seventeen are visibly indicated beside the preview and an
  explicit action expands them; collapsing restores the preview without losing matters
