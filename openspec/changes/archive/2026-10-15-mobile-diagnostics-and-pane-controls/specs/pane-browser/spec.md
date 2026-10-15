## ADDED Requirements

### Requirement: Mobile pane controls match the Mac metaphor
Phone and iPad All panes SHALL use the Mac's vertical expand/compress metaphor with
a centre rectangle and outward/inward arrows, changing its accessible action name with
fold state. Its action target SHALL be at least 44 points. Search SHALL use a decorative
vector magnifier at a readable 20-point size with a 44-point target that focuses the
existing search field. Search/filter, collapse persistence and keyboard behavior SHALL
remain unchanged.

#### Scenario: Search and fold on either shell
- **WHEN** the user taps the magnifier
- **THEN** the existing search field receives focus
- **WHEN** Expand all or Collapse all is tapped
- **THEN** all groups change accordingly and the icon/action label changes together
