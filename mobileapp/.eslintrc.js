module.exports = {
  root: true,
  extends: '@react-native',
  // e2e/ is a Node + webdriverio layer with its own tsconfig; ts-jest typechecks
  // it when the suite runs (npm run test:e2e). Keep it out of the RN lint pass.
  // src/ui/xtermAsset.ts is a generated ~300KB inline xterm.js bundle — don't lint it.
  ignorePatterns: ['e2e/', 'src/ui/xtermAsset.ts'],
};
