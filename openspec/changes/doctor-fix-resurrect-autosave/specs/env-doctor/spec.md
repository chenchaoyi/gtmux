## MODIFIED Requirements

### Requirement: Check the resurrect autosave is armed

When an installed continuum script exists but the running tmux status-right has
no save trigger, `doctor --fix` SHALL offer to add the script's absolute-path
trigger while retaining the existing status text. It SHALL back up the config,
persist a guarded append in the managed block, apply it live, and verify one
trigger is present. Existing and duplicate triggers SHALL remain untouched.

#### Scenario: Missing save trigger

- **WHEN** the installed continuum plugin has a save script but the current status-right has no trigger
- **THEN** `doctor --fix` offers to append one absolute-path trigger after consent
- **AND** sourcing the config again does not append a duplicate

#### Scenario: Live apply fails

- **WHEN** the config is written but the running tmux cannot be armed
- **THEN** the fixer reports failure instead of claiming autosave is enabled

### Requirement: An update reports what changed, in the user's terms

After a successful install, `gtmux update` SHALL print a localized reminder to
run `gtmux doctor`. It SHALL not run the full doctor probe automatically or print
the reminder for `--check` or a failed install.

#### Scenario: Update succeeds without release notes

- **WHEN** an update succeeds but release notes are unavailable
- **THEN** the doctor reminder still appears

### Requirement: Menu-bar app as its own section

Doctor and `doctor --fix` SHALL agree on the installed app path, searching
`~/Applications` before `/Applications`. An incomplete bundle lacking
`Contents/Info.plist` SHALL not hide a complete installation in the other path.

#### Scenario: App installed by Homebrew

- **WHEN** Gtmux.app is present only in `/Applications`
- **THEN** doctor reports it as installed and the fixer skips the app-install step
