# Menu presentation during app-initiated work

Ordinary menu openings remain AppKit transient popovers. Restore and automatic
upgrade notes protect only the current presentation from activation-driven dismissal.
`MenuPopoverPresentation` owns this narrow AppKit boundary: the view and AgentStore
retain their existing progress/data responsibilities; no restore or fetch is repeated.

Restore protects the visible menu before spawning the CLI. Reopening during an active
restore also protects that presentation. Completion updates the existing row and never
shows, activates or reopens the menu. Protection lasts until dismissal so completion
while a terminal is active does not hide the final state.

Automatic notes open only after the menu is shown. The reader is positioned beside it
where screen space permits; clicks in either window are allowed. If there is insufficient
horizontal space, both remain accessible with normal window placement. A late receipt
while hidden remains pending for the next opening. Manual notes, Preferences, Pairing,
focus and other explicit navigation retain their existing close-before-navigation policy.

Protected menus close on an outside mouse click, Escape within the menu, status-item
toggle or explicit navigation. Only mouse events are observed globally (no new permission
or global key capture); local events are not consumed except menu Escape. Companion
child windows count as inside. Closing removes event monitors and restores transient
behavior. Keyboard-only switching to another app can leave a protected menu visible;
an outside click or another explicit dismissal closes it. This is scoped to the current
presentation, not a global always-open preference.

Tests use synthetic AppKit windows and verify lifecycle, outside-click delivery,
companion allowance, Escape and late-callback behavior. An isolated preview must not
poll the CLI, restore actual sessions or acknowledge personal release notes.

Primary API references: [AppKit popover behavior](https://developer.apple.com/documentation/appkit/nspopover/behavior-swift.enum/applicationdefined)
and [global mouse monitoring](https://developer.apple.com/documentation/appkit/nsevent/addglobalmonitorforevents(matching:handler:)).
