module.exports = {
  preset: '@react-native/jest-preset',
  setupFiles: ['<rootDir>/jest.setup.js'],
  // Transform the RN ecosystem + community/navigation packages (they ship ESM/Flow).
  transformIgnorePatterns: [
    'node_modules/(?!(?:jest-)?@?react-native|@react-native-community|@react-native-async-storage|@react-navigation|react-native-.*)/',
  ],
};
