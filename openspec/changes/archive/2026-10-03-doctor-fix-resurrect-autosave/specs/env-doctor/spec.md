## MODIFIED Requirements

### Requirement: Check the resurrect autosave is armed

`gtmux doctor` SHALL, in its "Restore after reboot" section and only when the
tmux-continuum plugin is installed, check that the running tmux `status-right` carries
continuum's save trigger (the `continuum_save.sh` interpolation continuum relies on to
autosave). When the trigger is missing it SHALL recommend adding it, because a custom
`status-right` without it silently disables autosave — the save goes stale and a reboot
restores an ancient snapshot.
`doctor --fix` SHALL offer to append the installed plugin's absolute-path trigger
without replacing the existing status text, and SHALL persist an idempotent guard
in the backed-up managed config block. It SHALL apply the change to the running
tmux and report failure if that cannot be verified. Existing and duplicated
triggers SHALL remain untouched.

When an installed continuum script exists but the running tmux status-right has
no save trigger, `doctor --fix` SHALL offer to add the script's absolute-path
trigger while retaining the existing status text. It SHALL back up the config,
persist a guarded append in the managed block, apply it live, and verify one
trigger is present. Existing and duplicate triggers SHALL remain untouched.

#### Scenario: Autosave trigger present

- **WHEN** the continuum plugin is installed and `status-right` contains the `continuum_save` trigger
- **THEN** doctor reports the autosave as OK (shown as `installed`, one consistent
  install-state word with the hooks/plugins/app — not a bespoke `armed`/`wired`)

#### Scenario: Autosave trigger missing

- **WHEN** the continuum plugin is installed but `status-right` does not contain the trigger
- **THEN** doctor flags it with a recommendation to add the `continuum_save.sh` interpolation to `status-right`

#### Scenario: Fix a missing trigger

- **WHEN** the installed save script exists, status-right has no trigger, and the user accepts `doctor --fix`
- **THEN** the fix preserves existing status text, adds one absolute-path trigger now, and persists a guarded append in the managed config block
- **AND** reloading the config does not add a second trigger, including when continuum already injected one

#### Scenario: Missing save trigger

- **WHEN** the installed continuum plugin has a save script but the current status-right has no trigger
- **THEN** `doctor --fix` offers to append one absolute-path trigger after consent
- **AND** sourcing the config again does not append a duplicate

#### Scenario: Live apply fails

- **WHEN** the config is written but the running tmux cannot be armed
- **THEN** the fixer reports failure instead of claiming autosave is enabled

### Requirement: An update reports what changed, in the user's terms

After a successful self-update the system SHALL summarise what changed, and the summary
SHALL be written for the user rather than derived from commit subjects — a commit subject
addresses whoever reads the diff, and identifiers such as revision hashes, change numbers
and author handles are noise to someone who only wants to know whether something they rely
on moved. The summary SHALL aggregate EVERY version crossed, not only the newest, since a
user several releases behind is exactly the one for whom describing one release would be a
lie of omission. It SHALL be brief, and point to a fuller listing for the remainder. It
SHALL be SILENT when it has nothing to say or cannot fetch the notes: this runs after the
install already succeeded, and an error about it reads as though the update itself failed.
A release whose author wrote no user-facing note SHALL contribute nothing rather than
having one invented for it.
After a successful install, `gtmux update` SHALL remind the user to run
`gtmux doctor` to check the local setup. `--check` and failed installs SHALL NOT
print that reminder; update SHALL NOT run the full doctor probe automatically.

After a successful install, `gtmux update` SHALL print a localized reminder to
run `gtmux doctor`. It SHALL not run the full doctor probe automatically or print
the reminder for `--check` or a failed install.

#### Scenario: Several versions crossed

- **WHEN** a user updates across more than one release
- **THEN** the summary covers all of them, newest first

#### Scenario: More than fits

- **WHEN** more changes exist than the summary shows
- **THEN** it says how many remain and how to see them

#### Scenario: Update completes

- **WHEN** `gtmux update` successfully installs a release
- **THEN** it prints a localized prompt to run `gtmux doctor`, even if no release notes could be fetched

#### Scenario: Notes unavailable

- **WHEN** the notes cannot be fetched
- **THEN** the update reports success and prints no summary and no error

#### Scenario: Update succeeds without release notes

- **WHEN** an update succeeds but release notes are unavailable
- **THEN** the doctor reminder still appears

### Requirement: Menu-bar app as its own section

`gtmux doctor` SHALL report the menu-bar app in a dedicated "Menu-bar app" section (not
folded into "Agents & notifications") showing its install state, version, on-disk path, and
whether it is up to date with the CLI. Doctor, its fixer, and update SHALL use the same
install-location search (`~/Applications` before `/Applications`) so they cannot
disagree about whether the app is present.

Doctor and `doctor --fix` SHALL agree on the installed app path, searching
`~/Applications` before `/Applications`. An incomplete bundle lacking
`Contents/Info.plist` SHALL not hide a complete installation in the other path.

#### Scenario: App section detail

- **WHEN** the menu-bar app is installed
- **THEN** a "Menu-bar app" section reports its version + on-disk path, and flags it if it is
  behind the CLI

#### Scenario: App installed by Homebrew

- **WHEN** Gtmux.app is present only in `/Applications`
- **THEN** doctor reports it as installed and the fixer skips the app-install step
