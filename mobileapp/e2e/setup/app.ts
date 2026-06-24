import {execFileSync} from 'child_process';
import {existsSync, readFileSync} from 'fs';
import {join} from 'path';
import {getDriver} from './driver';

export const BUNDLE = 'com.gtmux.app';
const TARGET = process.env.GTMUX_E2E_UDID || 'booted';

/**
 * Relaunch the app with debug launch env. `mobile: launchApp` actually passes
 * the environment to the new process — `activateApp` would drop it. Use this to
 * drive the GTMUX_DEBUG_* layer (auto-pair, no-push, net logging).
 */
export async function launchWithEnv(env: Record<string, string>): Promise<void> {
  const driver = getDriver();
  try {
    await driver.terminateApp(BUNDLE);
  } catch {
    /* not running */
  }
  await driver.execute('mobile: launchApp', {bundleId: BUNDLE, environment: env});
}

/** Plain relaunch (no debug env) — clean process, persisted state. */
export async function relaunch(): Promise<void> {
  const driver = getDriver();
  try {
    await driver.terminateApp(BUNDLE);
  } catch {
    /* not running */
  }
  await driver.activateApp(BUNDLE);
}

/**
 * Read the structured debug log the app wrote (Documents/gtmux-debug.jsonl) via
 * `simctl get_app_container`. Returns parsed JSONL records (e.g. {event:'net',
 * method, path, status, ms}); [] if logging was off or the file is absent.
 */
export function readDebugLog(): Array<Record<string, unknown>> {
  let container: string;
  try {
    container = execFileSync('xcrun', ['simctl', 'get_app_container', TARGET, BUNDLE, 'data'], {
      encoding: 'utf8',
    }).trim();
  } catch {
    return [];
  }
  const path = join(container, 'Documents', 'gtmux-debug.jsonl');
  if (!existsSync(path)) return [];
  return readFileSync(path, 'utf8')
    .split('\n')
    .filter(Boolean)
    .map(line => {
      try {
        return JSON.parse(line) as Record<string, unknown>;
      } catch {
        return null;
      }
    })
    .filter((x): x is Record<string, unknown> => x != null);
}
