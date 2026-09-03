module.exports = {
  preset: '@react-native/jest-preset',
  setupFiles: ['<rootDir>/jest.setup.js'],
  // e2e/ has its own runner (e2e/jest.config.e2e.js, Appium/Node); keep it out
  // of the RN unit-test pass.
  // The Appium-driven suites under e2e/__tests__ need a simulator and are run by
  // jest.config.e2e.js. Everything else under e2e/ is ordinary code — the harness's own
  // helpers — and is covered here: it had neither tests nor lint when it leaked a
  // WebDriverAgent for 25 hours (see e2e/setup/reclaim.ts).
  testPathIgnorePatterns: ['/node_modules/', '/e2e/__tests__/'],
  // Transform the RN ecosystem + community/navigation packages (they ship ESM/Flow).
  transformIgnorePatterns: [
    'node_modules/(?!(?:jest-)?@?react-native|@react-native-community|@react-native-async-storage|@react-navigation|react-native-.*)/',
  ],
};
