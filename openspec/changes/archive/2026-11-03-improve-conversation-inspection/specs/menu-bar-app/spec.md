## ADDED Requirements

### Requirement: Visible restore outcomes and detached session meaning

The menu-bar Restore action SHALL retain in-progress exclusion and capture a bounded completion outcome off the main thread. Failure SHALL remain visible, including with an empty pending plan, and a new attempt SHALL clear the prior error. All-detached plans SHALL describe opening sessions in the terminal, distinguish them from saved missing sessions, and state that current sessions remain open.

#### Scenario: A restore command fails before opening a tab

- **WHEN** the application restore command exits unsuccessfully
- **THEN** the user sees a failure indication and can retry rather than receiving an unchanged silent button
