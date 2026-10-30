## ADDED Requirements

### Requirement: Bounded recorded context with audience isolation
The shared transcript reader SHALL preserve explicitly recorded known harness context
separately from human prompts, assistant prose and calls. It SHALL NOT ingest private
reasoning or tool results as context. Prefix records SHALL attach before the next real
prompt without creating a fake turn. Complete known harness envelopes in Codex/Claude
input, known Kimi origins and public DSH injection appends SHALL be supported;
unsupported logs SHALL NOT trigger reconstruction. Each record SHALL be UTF-8 bounded
to 8 KiB, and each turn to 32 context records / 64 KiB, with retained truncation marked.
Context SHALL count towards existing cache/response budgets. Compatible readers SHALL
opt in with context=1; older readers SHALL retain the original public-message segment
shape. Opted-in owner/full-scope paired transcripts MAY include additive segments[].context;
guest responses SHALL strip it
and empty context-only segments, with audience-separated conditional revisions.

#### Scenario: Append after prefix instructions
- **WHEN** a log contains only known prefix context and later receives a real user turn
- **THEN** incremental parsing retains the context once before the real prompt
- **AND** no context text becomes a human message or public response

#### Scenario: Read the same pane with owner and guest credentials
- **WHEN** owner, paired and guest readers request a context-bearing transcript
- **THEN** owner and paired readers receive context and the guest receives original public messages without context-only segments
- **AND** an owner conditional revision cannot reuse its representation for the guest

#### Scenario: Read examples and unsupported records
- **WHEN** text contains inline, fenced or incomplete harness examples, private thinking, tool output or an unsupported provider record
- **THEN** no context is reconstructed from those records

#### Scenario: Keep an older App readable during a server upgrade
- **WHEN** an owner client requests a transcript without the context capability query
- **THEN** context and empty context-only segments are omitted
- **AND** its conditional revision cannot reuse the opted-in reader representation
