## ADDED Requirements

### Requirement: Direct compact appearance settings
Browser settings SHALL expose System, Light and Dark as immediately selectable choices, without opening a menu. The control SHALL retain native radio keyboard and selection semantics, persisted preferences and system following. Settings SHALL avoid redundant singleton headings and keep labels, size sliders and short hints compact while preserving touch targets and viewport bounds.

#### Scenario: Switch appearance in one action
- **WHEN** a reader activates a visible appearance choice
- **THEN** the page applies it immediately, exposes its selected state and preserves it after reload
- **AND** selecting System resumes following system appearance
