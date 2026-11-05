import globalSetup from './global-setup';
const {setupManual, verifySendTarget} = require('./manual-tools.cjs');

// Validate before globalSetup can reclaim/start Appium or connect to a device.
export default async function manualGlobalSetup(): Promise<void> {
  await setupManual(process.env, verifySendTarget, globalSetup);
}
