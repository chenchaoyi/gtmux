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
/**
 * Make the loopback Appium connection bypass any HTTP proxy. On a machine with a
 * local proxy configured (e.g. a Clash `HTTP_PROXY=http://127.0.0.1:7897`) and no
 * NO_PROXY, webdriverio's undici client routes even the POST to Appium's
 * 127.0.0.1:4723 through the proxy, which resets it → `UND_ERR_SOCKET` on
 * `/session` (while a plain GET /status, not proxied, still works — the confusing
 * part). Adding loopback to NO_PROXY fixes it; no-op when no proxy is set.
 */
function ensureLoopbackNoProxy(): void {
  const proxied = ['HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'http_proxy', 'https_proxy', 'all_proxy'].some(
    k => (process.env[k] || '').trim() !== '',
  );
  if (!proxied) return;
  for (const key of ['NO_PROXY', 'no_proxy']) {
    const cur = process.env[key] || '';
    const has = cur
      .split(',')
      .map(s => s.trim())
      .some(h => h === '127.0.0.1' || h === 'localhost');
    if (!has) process.env[key] = [cur, '127.0.0.1', 'localhost'].filter(Boolean).join(',');
  }
}

export default async function globalSetup(): Promise<void> {
  ensureLoopbackNoProxy();
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

  let driver;
  try {
    driver = await remote({
      hostname: '127.0.0.1',
      port: appiumPort,
      path: '/',
      capabilities: iosCapabilities,
      logLevel: 'warn',
      connectionRetryTimeout: 360_000, // first run builds WebDriverAgent (slow — minutes on Intel)
    });
  } catch (err) {
    // If opening the session fails, jest SKIPS globalTeardown — so the Appium
    // server we just spawned would leak and hold :4723, breaking the next run.
    // Kill its process group before rethrowing.
    try {
      if (server.pid) process.kill(-server.pid, 'SIGKILL');
    } catch {
      /* already gone */
    }
    throw err;
  }
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
