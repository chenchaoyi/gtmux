## ADDED Requirements

### Requirement: Off-main CLI work, sleep awareness, and independent surface visibility
The menu-bar app SHALL read notification reply options, render notification badge attachments, refresh `tab-alert`, and poll `PaneBrowserStore` (`AgentRefreshGate` + bounded `capturePoll`) off the main thread. `AppDelegate` SHALL pause `awakeTimer` and `tabOrderTimer` while displays sleep, gate remote-client and share refreshes on popover visibility, and give the popover and `PaneBrowserController` independent `SurfaceVisibility` states so closing one surface never stops animations in the other.

#### Scenario: Closing All Panes while the popover stays open
- **WHEN** `PaneBrowserController` closes while the menu-bar popover is visible, or an input notification arrives
- **THEN** only the pane browser's `SurfaceVisibility` turns off, and `NotificationManager` resolves `gtmux options` off-main before posting on the main actor
