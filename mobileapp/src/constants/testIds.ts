// Stable accessibility identifiers for end-to-end UI tests (Appium / XCUITest).
// RN's `testID` prop maps to iOS `accessibilityIdentifier`, which Appium targets
// as `~<id>`. Sourcing the strings here means a rename refactors both the
// component and the e2e selector at once. Keep ids short, kebab-case, stable.

export const TestIds = {
  servers: {
    screen: 'servers-screen',
    add: 'servers-add',
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
  },
} as const;
