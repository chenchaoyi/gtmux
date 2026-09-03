/**
 * Closes the webdriverio session, group-kills the Appium server, then ends any
 * WebDriverAgent by predicate. Errors are logged, never thrown — teardown must always let
 * the runner exit.
 *
 * The predicate pass is not redundant with the group kill: by the time we get here the
 * WDA may already be re-parented, and a run that never gets here at all is exactly the
 * case setup's reclaim covers.
 */
import {reclaimAfter} from './reclaim';

export default async function globalTeardown(): Promise<void> {
  const driver = globalThis.__E2E_DRIVER__;
  if (driver) {
    try {
      await driver.deleteSession();
    } catch (err) {
      // eslint-disable-next-line no-console
      console.warn('[e2e] session teardown error (continuing):', err);
    }
    globalThis.__E2E_DRIVER__ = undefined;
  }

  const pid = globalThis.__E2E_SERVER_PID__;
  if (pid) {
    // Negative pid → kill the whole process group (catches the WebDriverAgent
    // xcodebuild grandchild). Requires detached:true at spawn.
    try {
      process.kill(-pid, 'SIGTERM');
    } catch {
      /* already gone */
    }
    await new Promise(r => setTimeout(r, 5_000));
    try {
      process.kill(-pid, 0); // probe — throws if the group is dead
      process.kill(-pid, 'SIGKILL');
    } catch {
      /* group already gone — good */
    }
    globalThis.__E2E_SERVER_PID__ = undefined;
  }

  reclaimAfter();
}
