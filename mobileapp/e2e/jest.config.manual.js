const {manualMode, sendTarget} = require('./setup/manual-tools.cjs');
const mode = manualMode(process.env); // Validate before globalSetup touches a simulator.
if (mode === 'diagnostics') sendTarget(process.env);
module.exports = {
  ...require('./jest.config.e2e.js'),
  globalSetup: '<rootDir>/e2e/setup/global-setup-manual.ts',
  testMatch: [`<rootDir>/e2e/${mode}/**/*.test.ts`],
};
