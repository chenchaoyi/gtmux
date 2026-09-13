# hq-knowledge (delta)

## ADDED Requirements

### Requirement: An entry carries its language and may carry the other

Every entry SHALL record the language it was written in; an entry MAY carry an alternate
half (title and body) in the other language, written by HQ — never machine-translated.
Entries written before this rule get their language inferred at read time and marked as
assumed; the ledger is not rewritten.

#### Scenario: Reading in the other language

- **WHEN** a reader whose language is English opens an entry written in Chinese that
  carries an English half
- **THEN** the English title and body are shown
- **AND WHEN** the entry carries no English half
- **THEN** the Chinese is shown with a language tag, and lint counts the entry as monolingual

### Requirement: Each output takes the language its readers have

Outputs SHALL render in the language their readers have: the machine's canonical file,
the agents' instruction blocks, a repository's block and the `LOCAL.md` landing in the
machine's language; the `everyone` brief and its issue prefill in English when an English
half exists.

#### Scenario: Promoting to everyone from a Chinese base

- **WHEN** an entry with an English half is promoted for everyone
- **THEN** the brief and the prefilled issue carry the English half
