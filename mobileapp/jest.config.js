module.exports = {
  preset: '@react-native/jest-preset',
  setupFiles: ['<rootDir>/jest.setup.js'],
  // e2e/ has its own runner (e2e/jest.config.e2e.js, Appium/Node); keep it out
  // of the RN unit-test pass.
  testPathIgnorePatterns: ['/node_modules/', '/e2e/'],
  // Transform the RN ecosystem + community/navigation packages (they ship ESM/Flow).
  transformIgnorePatterns: [
    'node_modules/(?!(?:jest-)?@?react-native|@react-native-community|@react-native-async-storage|@react-navigation|react-native-.*)/',
  ],
};
