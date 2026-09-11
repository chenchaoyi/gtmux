# hq-knowledge (delta)

## MODIFIED Requirements

### Requirement: Session transcripts are mined for candidates, LLM-free and incremental

The miner SHALL read Claude Code, Codex and Kimi Code sessions and the transcript gtmux
writes for opencode. One conversation model SHALL serve every agent — a reader
translates its agent's envelope into who spoke, what tool ran and what it printed, and
the judgement lives once. A reader SHALL claim only what its agent's log was observed to
carry.

#### Scenario: A Codex record the agent injected is not the human

- **WHEN** a Codex user-role message opens with one of Codex's own tags
  (`<environment_context>`, `<recommended_plugins>`, `<turn_aborted>`)
- **THEN** it is neither a human line nor a candidate
