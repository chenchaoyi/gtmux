/**
 * Jest config for end-to-end UI tests via Appium (XCUITest).
 *
 * Separate from the main jest.config.js (react-native preset, unit tests): e2e
 * is pure Node + ts-jest — no RN runtime, no react-test-renderer. The shared
 * Appium server + webdriverio session are created once in global-setup.ts.
 */
module.exports = {
  rootDir: '..',
  preset: 'ts-jest',
  testEnvironment: 'node',
  testMatch: ['<rootDir>/e2e/__tests__/**/*.test.ts'],
  testTimeout: 120_000,
  globalSetup: '<rootDir>/e2e/setup/global-setup.ts',
  globalTeardown: '<rootDir>/e2e/setup/global-teardown.ts',
  maxWorkers: 1, // one shared sim session; run serially
  transform: {
    '^.+\\.ts$': ['ts-jest', {tsconfig: '<rootDir>/e2e/tsconfig.json'}],
  },
};
