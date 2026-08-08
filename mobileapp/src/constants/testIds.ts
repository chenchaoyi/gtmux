// Stable accessibility identifiers for end-to-end UI tests (Appium / XCUITest).
// RN's `testID` prop maps to iOS `accessibilityIdentifier`, which Appium targets
// as `~<id>`. Sourcing the strings here means a rename refactors both the
// component and the e2e selector at once. Keep ids short, kebab-case, stable.

export const TestIds = {
  servers: {
    screen: 'servers-screen',
    add: 'servers-add',
    disconnect: 'servers-disconnect',
  },
  pairing: {
    screen: 'pairing-screen',
    scan: 'pairing-scan',
    host: 'pairing-host',
    token: 'pairing-token',
    connect: 'pairing-connect',
    error: 'pairing-error',
  },
  radar: {
    screen: 'radar-screen',
    serverChip: 'radar-server-chip',
    settings: 'radar-settings',
    waitingOnly: 'radar-waiting-only',
    panes: 'radar-panes',
  },
  panes: {
    screen: 'panes-screen',
    back: 'panes-back',
    search: 'panes-search',
    row: 'panes-row', // suffixed with the pane id → `${panes.row}-${paneId}`
    section: 'panes-section', // session header, suffixed with the session name (collapsible)
  },
  agent: {
    // one per row; suffixed with the pane id so a test can target a known agent
    row: 'agent-row', // use `${agent.row}-${paneId}`
  },
  detail: {
    screen: 'detail-screen',
    back: 'detail-back',
    pane: 'detail-pane',
    modeChat: 'detail-mode-chat',
    modeTerminal: 'detail-mode-terminal',
    chat: 'detail-chat',
    fullscreen: 'detail-fullscreen',
    fsExit: 'detail-fs-exit',
    collapseAll: 'detail-collapse-all',
    collapsedReply: 'detail-collapsed-reply',
    chatEarlier: 'detail-chat-earlier',
    chatThinking: 'detail-chat-thinking',
    jumpBottom: 'detail-jump-bottom',
  },
  settings: {
    // one per PickerSheet option; suffixed with the option key →
    // `${settings.pickerOption}-<key>` (e.g. `picker-option-en`)
    pickerOption: 'picker-option',
  },
  composer: {
    input: 'composer-input',
    send: 'composer-send',
    keyboard: 'composer-kbd',
    // one per resting-row control key; suffixed with the tmux key name it sends →
    // `${composer.controlKey}-Tab` / `-Up` / `-Down` / `-Enter` / `-BSpace` / `-C-c` / `-Escape`
    controlKey: 'composer-key',
    snippets: 'composer-snippets',
    snippetSheet: 'composer-snippet-sheet',
    history: 'composer-history',
    attach: 'composer-attach',
    expand: 'composer-expand',
    attachSheet: 'composer-attach-sheet',
  },
} as const;
