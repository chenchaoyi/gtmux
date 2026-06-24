import {spawn, ChildProcess} from 'child_process';
import {existsSync, mkdirSync, symlinkSync, unlinkSync, writeFileSync} from 'fs';
import {join, resolve} from 'path';
import {remote} from 'webdriverio';
import {appiumPort, appiumServerUrl, iosCapabilities} from './capabilities';
import {writeDebugFlags} from './app';

/**
 * Boots an Appium server (its own process group so teardown can kill the
 * WebDriverAgent xcodebuild grandchild), waits for /status, then opens a
 * webdriverio session against the booted iOS sim. Handles + artifacts dir are
 * stashed on globalThis for the tests and teardown to find.
 */
export default async function globalSetup(): Promise<void> {
  const appRoot = resolve(__dirname, '../..'); // mobileapp/
  const artifactsRoot = join(appRoot, '.e2e-artifacts');
  const runStamp = new Date().toISOString().replace(/[-:T]/g, '').replace(/\..+$/, '');
  const runDir = join(artifactsRoot, runStamp);
  mkdirSync(runDir, {recursive: true});

  const latestLink = join(artifactsRoot, 'latest');
  try {
    if (existsSync(latestLink)) unlinkSync(latestLink);
    symlinkSync(runDir, latestLink, 'dir');
  } catch {
    /* non-fatal */
  }
  globalThis.__E2E_ARTIFACTS_DIR__ = runDir;

  const appiumLogPath = join(runDir, 'appium.log');
  // eslint-disable-next-line no-console
  console.log(`[e2e] Spawning Appium server, log → ${appiumLogPath}`);
  const server: ChildProcess = spawn(
    'npx',
    ['appium', '--port', String(appiumPort), '--log', appiumLogPath, '--log-level', 'info'],
    {cwd: appRoot, detached: true, stdio: 'ignore'},
  );
  if (server.pid == null) throw new Error('[e2e] Appium failed to spawn (no PID)');
  writeFileSync(join(runDir, 'server.pid'), String(server.pid), 'utf8');
  globalThis.__E2E_SERVER_PID__ = server.pid;

  await waitForServerReady(60_000);
  // eslint-disable-next-line no-console
  console.log('[e2e] Appium ready; opening session…');

  // Baseline debug flags BEFORE the session's first app launch, so even that
  // launch has NO_PUSH — otherwise it requests notification permission and the
  // pending system prompt haunts every later launch, blocking UI interaction.
  // Requires the app already installed (npm run e2e:build). Best-effort.
  try {
    writeDebugFlags({GTMUX_DEBUG_NO_PUSH: '1'});
  } catch {
    /* app not installed yet — tests will surface it */
  }

  const driver = await remote({
    hostname: '127.0.0.1',
    port: appiumPort,
    path: '/',
    capabilities: iosCapabilities,
    logLevel: 'warn',
    connectionRetryTimeout: 180_000, // first run builds WebDriverAgent (slow)
  });
  globalThis.__E2E_DRIVER__ = driver;
  // eslint-disable-next-line no-console
  console.log('[e2e] Session ready:', driver.sessionId);
}

async function waitForServerReady(timeoutMs: number): Promise<void> {
  const start = Date.now();
  let lastErr: unknown;
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(`${appiumServerUrl}/status`);
      if (res.ok) return;
      lastErr = new Error(`HTTP ${res.status}`);
    } catch (err) {
      lastErr = err;
    }
    await new Promise(r => setTimeout(r, 250));
  }
  throw new Error(`[e2e] Appium not ready after ${timeoutMs}ms (last: ${lastErr})`);
}
