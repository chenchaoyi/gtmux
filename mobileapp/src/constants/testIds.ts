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
    filter: 'radar-filter',
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
  },
  composer: {
    input: 'composer-input',
    send: 'composer-send',
    keyboard: 'composer-kbd',
    snippets: 'composer-snippets',
    snippetSheet: 'composer-snippet-sheet',
    history: 'composer-history',
  },
} as const;
