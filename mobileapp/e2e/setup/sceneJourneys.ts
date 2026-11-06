import {execFileSync} from 'child_process';
import {existsSync, readFileSync, writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver, recordSceneStep} from './driver';
import {BUNDLE, launchWithFlags, settle, typeInto, waitForNotificationAuthorization, writeDebugFlags} from './app';
import {screenshot} from './screenshot';
import {Fake} from '../fake-serve/server';

export type SceneSurface = 'ordinary-phone' | 'regular-iPad' | 'duo';
export type SceneJourney = {fake: Fake; surface: SceneSurface; container?: string; steps: string[]};
type Rect = {x: number; y: number; width: number; height: number};
type Geometry = {pid: number; host: number; rootAttached: boolean; scenePresent: boolean;
  windowBounds: Rect; rootBounds: Rect; rootInWindow: Rect; insets: {top: number; right: number; bottom: number; left: number}};
type NativeEntry = {event: string; time: number; geometry?: Geometry};

function record(ctx: SceneJourney, step: string) {ctx.steps.push(step); recordSceneStep(step);}
function flags(ctx: SceneJourney, push: boolean) {
  return {GTMUX_DEBUG_SERVERS: JSON.stringify([{url: ctx.fake.url, token: ctx.fake.token, name: 'Fixture Mac'}]),
    GTMUX_DEBUG_SHOT_MODE: '1', GTMUX_DEBUG_LANG: 'en', GTMUX_DEBUG_LOG_NET: '1', ...(!push ? {GTMUX_DEBUG_NO_PUSH: '1'} : {})};
}
function requireDevice() {
  const udid = process.env.GTMUX_E2E_UDID;
  if (!udid || udid === 'booted') throw new Error('Scene migration requires an explicit simulator');
  return udid;
}
function nativeEntries(ctx: SceneJourney): NativeEntry[] {
  if (!ctx.container) throw new Error('No successful launch container for this journey');
  const path = join(ctx.container, 'Documents/gtmux-native-notification-trace.jsonl');
  if (!existsSync(path)) return [];
  return readFileSync(path, 'utf8').split('\n').filter(Boolean).flatMap(line => {
    try {return [JSON.parse(line) as NativeEntry];} catch {return [];}
  });
}
export async function assertSceneSurface(surface: SceneSurface) {
  const d = getDriver();
  const expected = surface === 'ordinary-phone' ? '~radar-screen' : '~radar-split';
  await d.$(expected).waitForDisplayed({timeout: 20000});
  if (surface === 'ordinary-phone') expect(await d.$('~radar-split').isExisting()).toBe(false);
}
function rectValid(rect: Rect) {
  return rect && [rect.x, rect.y, rect.width, rect.height].every(Number.isFinite) && rect.width > 0 && rect.height > 0;
}
export function geometryMatches(g: Geometry, root: Rect, size: {width: number; height: number}): boolean {
  if (!g || !g.rootAttached || !g.scenePresent || !rectValid(root) || !rectValid(g.windowBounds) || !rectValid(g.rootBounds) || !rectValid(g.rootInWindow)) return false;
  if (!g.insets || Object.values(g.insets).some(n => !Number.isFinite(n) || n < 0)
      || g.insets.left + g.insets.right >= size.width || g.insets.top + g.insets.bottom >= size.height) return false;
  const pairs = [[g.windowBounds.width, size.width], [g.windowBounds.height, size.height],
    [g.rootBounds.width, size.width], [g.rootBounds.height, size.height],
    [g.rootInWindow.x, root.x], [g.rootInWindow.y, root.y], [g.rootInWindow.width, root.width], [g.rootInWindow.height, root.height]];
  return pairs.every(([a, b]) => Math.abs(a - b) <= 1);
}
export async function observeGeometry(ctx: SceneJourney, since: number, host?: {pid: number; host: number}) {
  const d = getDriver();
  let previous = '', stable = 0;
  let result: {native: NativeEntry; root: Rect; window: {width: number; height: number}} | undefined;
  await d.waitUntil(async () => {
    const native = nativeEntries(ctx).filter(e => e.time > since && e.geometry?.rootAttached).at(-1);
    if (!native?.geometry) return false;
    if (host && (native.geometry.pid !== host.pid || native.geometry.host !== host.host)) throw new Error('Headless root was replaced before visual attachment');
    const root = await d.getElementRect(await d.$('~window-metrics-root').elementId);
    const window = await d.getWindowSize();
    if (!geometryMatches(native.geometry, root, window)) {stable = 0; return false;}
    const signature = JSON.stringify({root, window, host: native.geometry.host, pid: native.geometry.pid});
    stable = previous === signature ? stable + 1 : 1;
    previous = signature;
    result = {native, root, window};
    return stable >= 2;
  }, {timeout: 20000, interval: 500, timeoutMsg: 'fresh native and WDA content geometry did not agree'});
  return result!;
}

export function insideSceneSafeArea(g: Geometry, frame: Rect): boolean {
  return rectValid(frame) && frame.x >= g.insets.left - 1 && frame.y >= g.insets.top - 1
    && frame.x + frame.width <= g.windowBounds.width - g.insets.right + 1
    && frame.y + frame.height <= g.windowBounds.height - g.insets.bottom + 1;
}
export async function readerToolsAndDraft(ctx: SceneJourney, geometry: Geometry) {
  const d = getDriver();
  const row = d.$('~agent-row-%13');
  await row.waitForDisplayed({timeout: 10000}); await row.click();
  await d.$('-ios predicate string:label == "Show full title, 整理这周的发布清单"').waitForDisplayed({timeout: 10000});
  if (ctx.surface === 'ordinary-phone') await d.$('~detail-back').waitForDisplayed({timeout: 10000});
  const tools = d.$('~session-tools'); await tools.waitForDisplayed({timeout: 10000});
  record(ctx, 'reader-selected');
  await tools.click();
  const traceAction = d.$('~session-tools-menu-trace'); await traceAction.waitForDisplayed({timeout: 10000});
  expect(insideSceneSafeArea(geometry, await d.getElementRect(await d.$('~session-tools-menu').elementId))).toBe(true);
  await traceAction.click();
  const title = d.$('~trace-session-title'); await title.waitForDisplayed({timeout: 15000});
  expect(await title.getText()).toContain('整理这周的发布清单');
  await d.$('~trace-search-toggle').click(); await typeInto('trace-search', 'part 12');
  expect(await d.$('~trace-search').getValue()).toBe('part 12');
  await d.$('~trace-close').click(); record(ctx, 'trace-used');
  await tools.waitForDisplayed({timeout: 10000}); await tools.click();
  const diffAction = d.$('~session-tools-menu-diff'); await diffAction.waitForDisplayed({timeout: 10000}); await diffAction.click();
  const wrap = d.$('~diff-wrap'); await wrap.waitForDisplayed({timeout: 15000});
  expect(insideSceneSafeArea(geometry, await d.getElementRect(await wrap.elementId))).toBe(true); await wrap.click();
  const lines = d.$('~diff-lines'); await lines.waitForExist({timeout: 10000});
  await d.execute('mobile: scroll', {elementId: await lines.elementId, direction: 'down'});
  await d.$('~Close changes').click(); record(ctx, 'diff-used');
  await d.$('~detail-mode-chat').click();
  await typeInto('composer-input', 'scene migration draft');
  expect(await d.$('~composer-input').getValue()).toBe('scene migration draft');
  record(ctx, 'draft-entered');
}

export async function coldSceneAction(ctx: SceneJourney) {
  const udid = requireDevice();
  if (process.env.GTMUX_E2E_ACCEPT_ALERTS !== '1') throw new Error('Notification alert acceptance is required');
  const d = getDriver(), launch = flags(ctx, true);
  ctx.container = await launchWithFlags(launch);
  await waitForNotificationAuthorization();
  writeDebugFlags(launch); await d.terminateApp(BUNDLE);
  expect(await d.queryAppState(BUNDLE)).toBe(1);
  ctx.fake.world.recorded = [];
  const body = 'Scene cold action fixture', file = join(getArtifactsDir(), 'scene-cold-push.json');
  writeFileSync(file, JSON.stringify({aps: {alert: {title: 'Fixture Mac', body}, category: 'AGENT_WAITING_2', sound: 'default'}, pane: '%12', server: 'Fixture Mac'}));
  const sentAt = Date.now() / 1000;
  execFileSync('xcrun', ['simctl', 'push', udid, BUNDLE, file], {timeout: 15000, maxBuffer: 16384, killSignal: 'SIGKILL'});
  const banner = d.$(`-ios predicate string:label CONTAINS "${body}"`);
  await banner.waitForDisplayed({timeout: 15000});
  await d.execute('mobile: touchAndHold', {elementId: await banner.elementId, duration: 1.5});
  const action = d.$('-ios predicate string:label == "2"'); await action.waitForDisplayed({timeout: 10000});
  await screenshot('scene-cold-action-before'); await action.click();
  await d.waitUntil(() => ctx.fake.world.writesTo('/api/send').length > 0, {timeout: 20000, interval: 500});
  await settle(2000);
  expect(ctx.fake.world.writesTo('/api/send')).toEqual([expect.objectContaining({id: '%12', text: '2'})]);
  const headless = nativeEntries(ctx).filter(e => e.event === 'headless-host' && e.time > sentAt).at(-1)?.geometry;
  expect(headless).toBeDefined(); expect(headless!.scenePresent).toBe(false);
  record(ctx, 'cold-native-headless'); record(ctx, 'cold-fake-once');
  const attachAt = Date.now() / 1000; await d.activateApp(BUNDLE); await assertSceneSurface(ctx.surface);
  const before = await observeGeometry(ctx, attachAt, headless);
  record(ctx, 'same-host-attached'); record(ctx, 'fresh-root-before');
  await readerToolsAndDraft(ctx, before.native.geometry!);
  const original = await d.getOrientation();
  try {
    const next = original === 'PORTRAIT' ? 'LANDSCAPE' : 'PORTRAIT';
    const rotateAt = Date.now() / 1000; await d.setOrientation(next);
    const after = await observeGeometry(ctx, rotateAt, headless);
    expect(after.window).not.toEqual(before.window); expect(await d.getOrientation()).toBe(next);
    record(ctx, 'fresh-root-rotated');
    await d.$('-ios predicate string:label == "Show full title, 整理这周的发布清单"').waitForDisplayed({timeout: 10000});
    expect(await d.$('~composer-input').getValue()).toBe('scene migration draft');
    expect((await d.$$('XCUIElementTypeKeyboard')).length).toBeGreaterThan(0);
    record(ctx, 'draft-keyboard-retained');
    await screenshot('scene-host-rotated');
    if (ctx.surface === 'ordinary-phone') {await d.$('~detail-back').click(); await assertSceneSurface(ctx.surface);}
    record(ctx, 'returned-to-list');
    writeFileSync(join(getArtifactsDir(), 'scene-host-geometry.json'), JSON.stringify({headless, before, after}, null, 2));
  } finally {await d.setOrientation(original);}
}

export async function warmSceneAction(ctx: SceneJourney) {
  const udid = requireDevice();
  if (process.env.GTMUX_E2E_ACCEPT_ALERTS !== '1') throw new Error('Notification alert acceptance is required');
  const d = getDriver(); ctx.container = await launchWithFlags(flags(ctx, true));
  await waitForNotificationAuthorization(); ctx.fake.world.recorded = [];
  await d.background(-1);
  await d.waitUntil(async () => (await d.queryAppState(BUNDLE)) !== 4, {timeout: 15000, interval: 250});
  record(ctx, 'warm-background');
  const body = 'Scene warm action fixture', file = join(getArtifactsDir(), 'scene-warm-push.json');
  writeFileSync(file, JSON.stringify({aps: {alert: {title: 'Fixture Mac', body}, category: 'AGENT_WAITING_2', sound: 'default'}, pane: '%12', server: 'Fixture Mac'}));
  execFileSync('xcrun', ['simctl', 'push', udid, BUNDLE, file], {timeout: 15000, maxBuffer: 16384, killSignal: 'SIGKILL'});
  const banner = d.$(`-ios predicate string:label CONTAINS "${body}"`); await banner.waitForDisplayed({timeout: 15000});
  await d.execute('mobile: touchAndHold', {elementId: await banner.elementId, duration: 1.5});
  const action = d.$('-ios predicate string:label == "2"'); await action.waitForDisplayed({timeout: 10000}); await action.click();
  await d.waitUntil(() => ctx.fake.world.writesTo('/api/send').length > 0, {timeout: 20000, interval: 500});
  await settle(2000);
  expect(ctx.fake.world.writesTo('/api/send')).toEqual([expect.objectContaining({id: '%12', text: '2'})]);
  record(ctx, 'warm-fake-once'); await screenshot('scene-warm-action-after');
}

async function usageVisible(timeout: number) {
  const d = getDriver();
  await d.waitUntil(async () => {
    if (await d.$('~hq-usage-close').isDisplayed().catch(() => false)) return true;
    const alert = await d.getAlertText().catch(() => '');
    if (/^Open in [“"]gtmux[”"]\?$/.test(alert.trim())) await d.acceptAlert();
    return false;
  }, {timeout, interval: 500, timeoutMsg: 'usage sheet did not become visible'});
}
export async function usageSceneLink(ctx: SceneJourney, cold: boolean) {
  requireDevice(); const d = getDriver(), launch = flags(ctx, false);
  ctx.container = await launchWithFlags(launch); await assertSceneSurface(ctx.surface);
  if (!cold) await d.$('-ios predicate string:label BEGINSWITH "Connection: connected"').waitForExist({timeout: 25000});
  else {writeDebugFlags(launch); await d.terminateApp(BUNDLE); expect(await d.queryAppState(BUNDLE)).toBe(1); record(ctx, 'cold-not-running');}
  const event = cold ? 'scene-url-cold' : 'scene-url';
  const before = nativeEntries(ctx).filter(e => e.event === event).length;
  await d.execute('mobile: deepLink', {url: 'gtmux://usage', ...(!cold ? {bundleId: BUNDLE} : {})});
  if (cold) {
    await usageVisible(20000);
    expect(nativeEntries(ctx).filter(e => e.event === event).length).toBeGreaterThan(before);
  } else {
    await d.waitUntil(() => nativeEntries(ctx).filter(e => e.event === event).length > before, {timeout: 20000, interval: 500});
    await usageVisible(15000);
  }
  record(ctx, `${cold ? 'cold' : 'warm'}-native-new-scene-url`); record(ctx, `${cold ? 'cold' : 'warm'}-usage-visible`);
  await screenshot(`scene-${cold ? 'cold' : 'warm'}-usage`);
}

export async function captureSceneJourney(ctx: SceneJourney, label: string) {
  const dir = getArtifactsDir();
  // No subprocess in evidence collection: reuse only this successful launch's
  // container. Absence is preserved as missing evidence, never another app's path.
  writeFileSync(join(dir, `${label}-steps.json`), JSON.stringify(ctx.steps));
  writeFileSync(join(dir, `${label}-fake.json`), JSON.stringify({requests: ctx.fake.requests, writes: ctx.fake.world.recorded}, null, 2));
  if (ctx.container) for (const [file, name] of [['gtmux-native-notification-trace.jsonl', 'native'], ['gtmux-debug.jsonl', 'js']]) {
    const source = join(ctx.container, 'Documents', file);
    if (existsSync(source)) writeFileSync(join(dir, `${label}-${name}.jsonl`), readFileSync(source));
  }
  await screenshot(`${label}-final`);
  writeFileSync(join(dir, `${label}-final.xml`), await getDriver().getPageSource());
}
