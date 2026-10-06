import {execFileSync} from 'child_process';
import {existsSync, mkdirSync, readFileSync, writeFileSync} from 'fs';
import {join} from 'path';
import {getDriver} from './driver';
import {TestIds} from '../../src/constants/testIds';

export const BUNDLE = 'com.gtmux.app';
const TARGET = process.env.GTMUX_E2E_UDID || 'booted';

function dataContainer(): string {
  return execFileSync('xcrun', ['simctl', 'get_app_container', TARGET, BUNDLE, 'data'], {
    encoding: 'utf8',
  }).trim();
}

/**
 * One view-state reset token per test FILE (F18). Jest loads this module afresh for each
 * file, so a file's first launch starts from the fixture view state (the app clears the
 * keys in state/uiState E2E_FIXTURE_KEYS once per token), and later launches in the same
 * file, however they relaunch, keep what it set. Without it, radar-refresh-collapsed left
 * every radar section folded and the next file could not find its rows. A file that must
 * inherit the state can pass GTMUX_DEBUG_RESET_UI_STATE: ''.
 */
const FILE_RESET_TOKEN = `${Date.now()}-${Math.random().toString(36).slice(2)}`;

/**
 * Drive the GTMUX_DEBUG_* layer by writing a flags FILE the app reads at startup
 * (Documents/gtmux-debug-flags.json), then cold-relaunching. This is deterministic
 * — unlike XCUITest's launchEnvironment (mobile: launchApp), which WDA caches per
 * session and applies unreliably on relaunch. Keys are the full GTMUX_DEBUG_*
 * names (e.g. {GTMUX_DEBUG_NO_PUSH: '1'}).
 */
export function writeDebugFlags(flags: Record<string, string>): void {
  const docs = join(dataContainer(), 'Documents');
  mkdirSync(docs, {recursive: true});
  const all = {GTMUX_DEBUG_RESET_UI_STATE: FILE_RESET_TOKEN, ...flags};
  writeFileSync(join(docs, 'gtmux-debug-flags.json'), JSON.stringify(all), 'utf8');
}

export async function launchWithFlags(flags: Record<string, string>): Promise<void> {
  // The What's New popup is skipped unless a suite asks for it with
  // GTMUX_DEBUG_SKIP_WHATS_NEW: '0': after a version bump it covered every suite's first
  // screen (findings F8). The app records the version as seen, as a fresh install does.
  writeDebugFlags({GTMUX_DEBUG_SKIP_WHATS_NEW: '1', ...flags});
  const driver = getDriver();
  try {
    await driver.terminateApp(BUNDLE);
  } catch {
    /* not running */
  }
  await driver.activateApp(BUNDLE);
}

/**
 * Tap the first agent row → Detail, returning true on success. Retries the
 * find→tap→assert as a unit: the radar re-renders every ~1.5s (SSE), which can
 * stale the element ref or make a tap miss. `detail-back` (a button) is the
 * reliable "we're on Detail" signal — a plain <View> like detail-pane is
 * reported visible=false by XCUITest.
 */
export async function openFirstAgentDetail(): Promise<boolean> {
  return openDetail(`-ios predicate string:name BEGINSWITH '${TestIds.agent.row}-'`);
}

/**
 * Open one radar row's Detail by its pane id. Against the fake the ids are known, so a
 * suite opens the pane it seeded rather than whichever row the radar happens to put
 * first. Same retry as openFirstAgentDetail, for the same reason.
 */
export async function openAgentDetail(paneId: string): Promise<boolean> {
  return openDetail(`~${TestIds.agent.row}-${paneId}`);
}

async function openDetail(rowSel: string): Promise<boolean> {
  const driver = getDriver();
  const back = driver.$(`~${TestIds.detail.back}`);
  for (let i = 0; i < 3; i++) {
    try {
      const row = driver.$(rowSel);
      await row.waitForDisplayed({timeout: 10_000});
      await settle(800); // let the list settle so the tap doesn't land mid-render
      await row.click();
      await back.waitForDisplayed({timeout: 6_000});
      return true;
    } catch {
      /* re-render race or missed tap — retry */
    }
  }
  return false;
}

/** Small fixed wait for an animation/transition to finish before interacting. */
export function settle(ms = 800): Promise<void> {
  return new Promise(r => setTimeout(r, ms));
}

/**
 * Type into a text field by testID, robustly: focus, set, and verify it stuck —
 * XCUITest `setValue` can silently no-op if the field isn't focused yet (e.g. a
 * modal still animating). Retries up to 3×. Secure fields can't be read back, so
 * pass {secure:true} to skip verification.
 */
export async function typeInto(testId: string, text: string, opts: {secure?: boolean} = {}): Promise<void> {
  const driver = getDriver();
  const el = driver.$(`~${testId}`);
  await el.waitForDisplayed({timeout: 10_000});
  for (let i = 0; i < 3; i++) {
    await el.click();
    await settle(250);
    try {
      await el.clearValue();
    } catch {
      /* already empty */
    }
    await el.setValue(text);
    await settle(150);
    if (opts.secure) return;
    const v = await el.getValue().catch(() => '');
    if (v === text) return;
  }
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

/**
 * Answer the alert on screen by tapping one of its buttons, and say so in the run's log.
 * Under GTMUX_E2E_ALERTS=manual nothing else touches an alert (capabilities.alertMode), so
 * every alert a test gets past is one it answered on the record: a system permission
 * prompt as much as the app's own confirmation. Returns the alert's title.
 */
export async function answerAlert(button: string, timeout = 5000): Promise<string> {
  const driver = getDriver();
  const alert = driver.$('-ios class chain:**/XCUIElementTypeAlert');
  await alert.waitForExist({timeout});
  const title = (await alert.getAttribute('label')) ?? '';
  await driver.$(`-ios class chain:**/XCUIElementTypeAlert/**/XCUIElementTypeButton[\`label == "${button}"\`]`).click();
  // eslint-disable-next-line no-console
  console.log(`[e2e] alert "${title}": tapped "${button}"`);
  return title;
}
