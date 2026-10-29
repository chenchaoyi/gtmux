# codex-terminal-identity Specification

## Purpose
TBD - created by archiving change codex-terminal-identity. Update Purpose after archive.

## Requirements

### Requirement: Codex terminal attachment is per frontend
The core SHALL associate an eligible shared-server Codex TUI with its own successful root thread start, resume or fork response after verifying local socket peer identity, frontend PID/birth, terminal, pane incarnation and Codex home. Backend PID, inherited environment, cwd, title, loaded threads and unsolicited notifications SHALL NOT establish ownership. Ephemeral helpers and subagent views SHALL NOT replace the root binding. Reordered responses and older connections SHALL NOT reclaim a newer attachment. Expired or exited attachments SHALL lose live authority while saved history remains restorable.

#### Scenario: Two ordinary clients share one backend and directory
- **WHEN** each verified frontend starts a different root thread
- **THEN** each pane retains its own thread without requiring the first message or a task nonce
- **AND** `/new` and explicit resume update only the frontend that requested them

#### Scenario: A frontend exits or a PID is reused
- **WHEN** its heartbeat expires, bridge exits or process/terminal incarnation changes
- **THEN** live input and role authority are withheld without deleting history or stopping the shared daemon

### Requirement: Launch integration preserves native behavior
The system SHALL use capability admission rather than a version threshold. Managed ordinary tmux-shell integration SHALL be removable and preserve existing aliases/functions. Spawn, HQ and restore SHALL share the adapter. Older embedded clients, explicit no-daemon/remote/custom/profile/config/headless or unsupported launches SHALL retain original arguments, environment, stdin and native exit behavior. A missing supported local daemon MAY be started only as part of an explicit eligible interactive launch using the official start operation. Discovery SHALL NOT start or resume agents.

#### Scenario: Custom shell launcher or older Codex
- **WHEN** the user has a Codex alias/function or a launch lacks bridge capabilities
- **THEN** the custom launch remains unchanged or executes the original Codex command without adding remote flags

### Requirement: Executing thread identity governs agent tools
The system SHALL use the executing Codex thread and canonical home instead of a shared backend's inherited pane. A pane or HQ role SHALL require a matching current binding. Invalid, ambiguous, subagent or unbound identities SHALL NOT borrow another pane or HQ authority. Reports MAY remain thread-only; replies SHALL target the saved exact conversation, or remain pull-readable when the original route is unavailable. Native reply attempts SHALL be claimed before mutation and SHALL NOT replay after uncertain acceptance. Independent legacy replies SHALL require matching original process birth and foreground binding. HQ event consumption and echo suppression SHALL use the same caller policy.

#### Scenario: Worker inherits HQ or another worker environment
- **WHEN** its runtime thread belongs to another verified pane
- **THEN** reports and audit records identify its own pane and it cannot claim HQ operations

#### Scenario: The source pane is reused before a reply
- **WHEN** HQ replies to a saved thread-scoped request
- **THEN** the new occupant receives nothing; exact native acceptance or a pull-readable uncertain/unavailable receipt is retained

### Requirement: Managed Codex HQ wakes retain exact input identity
HQ wake delivery SHALL use the verified addressed route for managed Codex, retaining existing input leases and draft guards. Accepted queue receipts MAY close a batch without claiming execution. Uncertain native batch entries SHALL be retained for reconciliation and degradation reporting, never recoalesced into a new message ID or replayed through a terminal. Legacy agents SHALL retain their existing wake protocol.

#### Scenario: Native wake acceptance is lost
- **WHEN** a managed Codex wake may have been accepted but has no confirmed receipt
- **THEN** its entries remain separately retained, repeated drains do not resend them, and degradation reporting includes overdue entries

### Requirement: Root workspace and child hook identity stay distinct
The adapter SHALL preserve implicit launch cwd and explicit directory overrides. It SHALL record the successful root response's workspace separately from the physical terminal target. Verified backend hooks SHALL use exact thread/home/frontend identity even when those directories differ. Codex hooks carrying a subagent agent_id SHALL NOT modify the parent's profile, lifecycle, input binding or HQ wake state.

#### Scenario: Resume a workspace different from the terminal shell
- **WHEN** the root workspace differs from its supervising shell's cwd
- **THEN** the exact frontend retains its root hooks and restore workspace without becoming an external session

#### Scenario: A child hook carries its parent's session ID
- **WHEN** a Codex hook also carries agent_id
- **THEN** it does not complete the parent turn or overwrite its profile
