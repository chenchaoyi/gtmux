## ADDED Requirements

### Requirement: Demonstration fixtures form a coherent isolated world
The offline demonstration SHALL reuse the production phone/iPad views with synthetic data only. Permission approval and refusal SHALL have distinct outcomes. Radar, terminal, chat, HQ assessment and pane titles SHALL agree after a decision. A client SHALL retain its session start timestamps across reads, and exiting SHALL cancel its pending scripted transitions. Usage identities and totals, knowledge topic counts and bodies SHALL agree with the sampled content. The tour SHALL include Codex segmented replies and a public desktop conversation shown through the production read-only view, without enabling desktop input or real host administration. The fake HTTP screenshot seed SHALL reuse these data sources.

#### Scenario: Rejecting the sample permission
- **WHEN** the user chooses not to run the sample tests
- **THEN** no test success is reported and HQ no longer asks to approve that permission

#### Scenario: Approving the sample permission
- **WHEN** the user approves the sample tests
- **THEN** the session first works and later finishes, with a matching transcript, assessment, pane title and completion indicator

#### Scenario: Browsing a public Codex sample
- **WHEN** the user opens the sample desktop conversation
- **THEN** the existing read-only conversation view displays synthetic replies and no composer or follow administration
