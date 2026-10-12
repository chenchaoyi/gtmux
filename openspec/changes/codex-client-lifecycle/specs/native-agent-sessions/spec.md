## ADDED Requirements

### Requirement: Separate Codex creation source from current control evidence
The system SHALL retain verified creation metadata separately from current attachment
and authorized input-route evidence. It SHALL NOT infer control from originator, cwd,
title, saved history, a lock filename, loaded status or direct-input capability alone.
Missing evidence SHALL remain unknown.

#### Scenario: CLI-created conversation continues in Desktop
- **WHEN** a CLI-created conversation has verified new Desktop activity
- **THEN** creation source SHALL remain CLI and current control evidence SHALL reflect Desktop
- **AND** old HQ dispatch role and automatic input/notification/knowledge authority SHALL NOT revive

#### Scenario: Backend cannot identify the frontend
- **WHEN** a thread is loaded but its frontend input route cannot be verified
- **THEN** authorized reading SHALL remain available
- **AND** unverified takeover/input SHALL remain unavailable

### Requirement: Retire only obsolete provisional native evidence
The system SHALL reconcile provisional native observations against exact tmux binding
and successful resource retirement. Conditional retirement SHALL protect newer
independent native activity, expose write failures and preserve persisted history.

#### Scenario: Bound worker is reaped after pane-less start
- **WHEN** provisional observation is followed by verified tmux binding and successful owned-resource reap
- **THEN** obsolete observation SHALL NOT resurface as a live native agent
- **AND** stored history SHALL remain without automatic archive/delete

#### Scenario: Fresh native activity races retirement
- **WHEN** a newer independent native observation arrives before retirement is written
- **THEN** conditional retirement SHALL preserve it
- **AND** the old dispatch SHALL remain ended without authority over the fresh client

### Requirement: Distinct Codex identities across trees and hosts
The system SHALL distinguish conversations by server, Codex home and verified thread
ID. Session-tree roots and identical titles SHALL NOT collapse forks. Backend restart
SHALL NOT duplicate history. Existing session_id compatibility SHALL retain its
verified thread-ID meaning.

#### Scenario: Forks share root and title
- **WHEN** threads share a tree root/title but have different thread IDs
- **THEN** they SHALL remain distinct with explicit lineage

#### Scenario: Backend reconnects
- **WHEN** backend changes but server/home/thread remain the same
- **THEN** history SHALL retain its identity and runtime attachments SHALL reconcile separately
