import type {Browser} from 'webdriverio';

/**
 * Singleton accessor for the shared driver + artifacts dir. The Browser is
 * created in global-setup.ts and stashed on globalThis so test files can reach
 * it without re-creating a session.
 */
declare global {
  // eslint-disable-next-line no-var
  var __E2E_DRIVER__: Browser | undefined;
  // eslint-disable-next-line no-var
  var __E2E_ARTIFACTS_DIR__: string | undefined;
  // eslint-disable-next-line no-var
  var __E2E_SERVER_PID__: number | undefined;
}

export function getDriver(): Browser {
  if (!globalThis.__E2E_DRIVER__) {
    throw new Error('[e2e] No driver — did global-setup.ts run? Import driver.ts only from e2e tests.');
  }
  return globalThis.__E2E_DRIVER__;
}

export function getArtifactsDir(): string {
  if (!globalThis.__E2E_ARTIFACTS_DIR__) {
    throw new Error('[e2e] Artifacts dir not set by global-setup.ts');
  }
  return globalThis.__E2E_ARTIFACTS_DIR__;
}
