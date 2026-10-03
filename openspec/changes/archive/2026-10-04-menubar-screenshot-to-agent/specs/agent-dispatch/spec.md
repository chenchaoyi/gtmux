## ADDED Requirements

### Requirement: `gtmux send --attach` hands a file to an agent by path

`gtmux send` SHALL accept `--attach FILE`, repeatable, for regular files up to 30 MB. After
the pane is found, each file SHALL be copied into gtmux's uploads dir (pruned by age and
size like the phone's uploads) under a name made of a hash of its content and its sanitized
base name, reusing an identical existing copy, and its absolute path SHALL be appended to
the message on a line of its own. The same file and message SHALL therefore produce the
same payload on a retry, so the re-send interlock recognises it. `--json` SHALL list the
paths as `attachments`. `--attach` SHALL be refused with `--key`, and nothing SHALL be
copied when the pane does not exist.

#### Scenario: A note and a screenshot

- **WHEN** `gtmux send %5 --message-file note.txt --attach "Screen Shot.png"` runs
- **THEN** the pane receives the note, then the copied file's path on its own line, with
  no space in the copied name

#### Scenario: The same screenshot sent twice

- **WHEN** the same file and note are sent again within the interlock window
- **THEN** the payload is identical and the second send is refused as a duplicate
