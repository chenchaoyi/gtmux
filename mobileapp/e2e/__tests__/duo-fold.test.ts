import {execFileSync} from 'child_process';
import {existsSync, readFileSync, writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver} from '../setup/driver';
import {launchWithFlags, readDebugLog, typeInto} from '../setup/app';
import {captureOnFailure, screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';
import {TestIds} from '../../src/constants/testIds';
import {createJourneyGuard} from '../setup/journeyGuard';

type Display = {id: number; metrics: {widthPixels: number; heightPixels: number}};
let fake: Fake;
beforeAll(async () => {
  const started = Date.now();
  console.log(`[e2e] Duo fixture begin ${started}`);
  fake = await startFake();
  fake.world.seedLongChat('%13', 12);
  fake.world.agents.find(agent => agent.pane_id === '%13')!.branch = 'main';
  fake.world.diffs.set('%13', 'diff --git a/duo.txt b/duo.txt\n--- a/duo.txt\n+++ b/duo.txt\n@@ -1 +1 @@\n-old\n+new\n');
  console.log(`[e2e] Duo fixture ready after ${Date.now() - started}ms`);
});
afterAll(async () => { await fake?.close(); });

// The real Appium commands set a simulator hinge, then enumerate and target the
// current panels. Pixel dimensions identify panels only, not application points.
async function posture(angle: number, panel: 'inner' | 'outer', evidence: object[], modalTarget?: string) {
    const driver = getDriver();
    const observation: {requestedAngle: number; panel: string; measured?: number; lastFrame?: object; selected?: number} = {requestedAngle: angle, panel};
    evidence.push(observation);
    await driver.execute('mobile: setSimulatedHingeAngle', {angle});
    let measured = NaN;
    // The documented command requests a transition; it does not guarantee
    // that a reading made immediately afterward has already settled.
    await driver.waitUntil(async () => {
      measured = await driver.execute('mobile: getSimulatedHingeAngle') as number;
      observation.measured = measured;
      return Math.abs(measured - angle) <= 5;
    }, {timeout: 20000, interval: 500, timeoutMsg: `hinge did not settle near ${angle}°`});
    const displays = await driver.execute('mobile: listDisplays') as Display[];
    expect(displays.length).toBeGreaterThanOrEqual(2);
    const sorted = [...displays].sort((a, b) =>
      a.metrics.widthPixels * a.metrics.heightPixels - b.metrics.widthPixels * b.metrics.heightPixels);
    const selected = panel === 'inner' ? sorted[sorted.length - 1] : sorted[0];
    observation.selected = selected.id;
    await driver.updateSettings({currentDisplayId: selected.id});
    const target = driver.$(modalTarget ?? '~window-metrics-root');
    await target.waitForExist({timeout: 15000});
    let previous = '', stable = 0;
    let size = {width: 0, height: 0};
    await driver.waitUntil(async () => {
      if (modalTarget && !await target.isDisplayed()) return false;
      const frame = await driver.getElementRect(await target.elementId);
      const bounds = modalTarget ? await driver.getWindowSize() : frame;
      observation.lastFrame = {frame, bounds};
      size = {width: bounds.width, height: bounds.height};
      const inside = frame.width > 0 && frame.height > 0 && (!modalTarget ||
        (frame.x >= 0 && frame.y >= 0 && frame.x + frame.width <= bounds.width && frame.y + frame.height <= bounds.height));
      const current = JSON.stringify({frame, bounds});
      stable = inside && current === previous ? stable + 1 : 0;
      previous = current;
      return stable >= 2;
    }, {timeout: 20000, interval: 500, timeoutMsg: 'scene layout did not settle on the selected display'});
    evidence.push({angle, measured, displays, selected: selected.id, size});
    await screenshot(`duo-fold-${panel}`);
    return size;
}

// A timed-out driver operation must finish cleanup before another case starts.
const guard = createJourneyGuard();
afterEach(guard.wait, 930000);

async function selectLongChat(evidence: object[]) {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en', GTMUX_DEBUG_LOG_NET: '1'});
  const initialSize = await posture(180, 'inner', evidence);
  const row = driver.$('~agent-row-%13');
  await row.waitForExist({timeout: 25000});
  const rowFrame = await driver.getElementRect(await row.elementId);
  expect(rowFrame.width).toBeGreaterThan(0);
  expect(rowFrame.height).toBeGreaterThan(0);
  expect(rowFrame.x).toBeGreaterThanOrEqual(0);
  expect(rowFrame.y).toBeGreaterThanOrEqual(0);
  expect(rowFrame.x + rowFrame.width).toBeLessThanOrEqual(initialSize.width);
  expect(rowFrame.y + rowFrame.height).toBeLessThanOrEqual(initialSize.height);
  await driver.execute('mobile: tap', {x: rowFrame.x + rowFrame.width / 2, y: rowFrame.y + rowFrame.height / 2});
  const chat = driver.$(`~${TestIds.detail.modeChat}`);
  await chat.waitForDisplayed({timeout: 10000});
  await chat.click();
  const selected = driver.$('~Show full title, 整理这周的发布清单');
  await selected.waitForDisplayed({timeout: 10000});
  return {initialSize, selected};
}

async function restorePosture() {
  const driver = getDriver();
  await driver.execute('mobile: setSimulatedHingeAngle', {angle: 180}).catch(() => {});
  await driver.updateSettings({currentDisplayId: null}).catch(() => {});
}

it('retains the selected conversation and reading position across simulator fold and unfold', () => guard.run(async () => {
  const driver = getDriver();
  const evidence: object[] = [];
  try {
    const {initialSize, selected} = await selectLongChat(evidence);
    const outerSize = await posture(0, 'outer', evidence);
    const readerSize = await driver.$('~workspace-reader').getSize();
    expect(Math.abs(readerSize.width - outerSize.width)).toBeLessThanOrEqual(1);
    await selected.waitForDisplayed({timeout: 15000});
    await posture(180, 'inner', evidence);
    await selected.waitForDisplayed({timeout: 15000});
    const chatView = driver.$(`~${TestIds.detail.chat}`);
    await chatView.waitForDisplayed({timeout: 10000});
    const canvas = {...await chatView.getLocation(), ...await chatView.getSize()};
    const x = Math.round(canvas.x + canvas.width / 2);
    await driver.action('pointer', {parameters: {pointerType: 'touch'}})
      .move({x, y: Math.round(canvas.y + canvas.height * 0.35)}).down().pause(80)
      .move({x, y: Math.round(canvas.y + canvas.height * 0.75), duration: 350}).up().perform();
    const jump = driver.$(`~${TestIds.detail.jumpBottom}`);
    await jump.waitForDisplayed({timeout: 10000});
    const prompts = await driver.$$('-ios predicate string:label BEGINSWITH "continue with part " AND visible == 1').getElements();
    expect(prompts.length).toBeGreaterThan(0);
    const anchor = await prompts[0].getAttribute('label');
    const anchoredPrompt = () => driver.$(`-ios predicate string:label == "${anchor}" AND visible == 1`);
    await screenshot('duo-fold-reading-inner');
    const readingOuter = await posture(0, 'outer', evidence);
    expect([readingOuter.width, readingOuter.height]).not.toEqual([initialSize.width, initialSize.height]);
    await jump.waitForDisplayed({timeout: 15000});
    await anchoredPrompt().waitForDisplayed({timeout: 15000});
    await screenshot('duo-fold-reading-outer');
    await posture(180, 'inner', evidence);
    await jump.waitForDisplayed({timeout: 15000});
    await anchoredPrompt().waitForDisplayed({timeout: 15000});
  } catch (error) {
    await captureOnFailure('duo-fold-failed', error);
  } finally {
    writeFileSync(join(getArtifactsDir(), 'duo-fold-postures.json'), JSON.stringify(evidence, null, 2));
    writeFileSync(join(getArtifactsDir(), 'duo-fold-reflow-debug.json'), JSON.stringify(readDebugLog().filter(item => String(item.event).startsWith('chat-reflow-')), null, 2));
    await restorePosture();
  }
}), 300000);

it('retains the selected conversation and keyboard draft across simulator fold and unfold', () => guard.run(async () => {
  const driver = getDriver();
  const evidence: object[] = [];
  try {
    const {selected} = await selectLongChat(evidence);
    const keyboard = driver.$(`~${TestIds.composer.keyboard}`);
    await keyboard.waitForDisplayed({timeout: 10000});
    await keyboard.click();
    const input = driver.$(`~${TestIds.composer.input}`);
    await input.waitForDisplayed({timeout: 10000});
    await input.setValue('Duo fold draft');
    expect(await input.getValue()).toBe('Duo fold draft');
    expect(await driver.$$('XCUIElementTypeKeyboard')).not.toHaveLength(0);
    const inner = await driver.$('~window-metrics-root').getSize();
    await screenshot('duo-fold-inner-draft');

    const outer = await posture(0, 'outer', evidence);
    expect([outer.width, outer.height]).not.toEqual([inner.width, inner.height]);
    await input.waitForDisplayed({timeout: 15000});
    expect(await input.getValue()).toBe('Duo fold draft');
    expect(await driver.$$('XCUIElementTypeKeyboard')).not.toHaveLength(0);
    await screenshot('duo-fold-outer-draft');

    // Keyboard presentation deliberately hides the header from accessibility.
    // Close it to verify the actual session and mode, then reopen the same draft.
    await keyboard.click();
    await selected.waitForDisplayed({timeout: 15000});
    await driver.$(`~${TestIds.detail.modeChat}`).waitForDisplayed({timeout: 15000});
    await keyboard.click();
    await input.waitForDisplayed({timeout: 15000});
    expect(await input.getValue()).toBe('Duo fold draft');
    expect(await driver.$$('XCUIElementTypeKeyboard')).not.toHaveLength(0);
    await posture(180, 'inner', evidence);
    await input.waitForDisplayed({timeout: 15000});
    expect(await input.getValue()).toBe('Duo fold draft');
    expect(await driver.$$('XCUIElementTypeKeyboard')).not.toHaveLength(0);
    await screenshot('duo-fold-inner-restored');
    await keyboard.click();
    await selected.waitForDisplayed({timeout: 15000});
    await driver.$(`~${TestIds.detail.modeChat}`).waitForDisplayed({timeout: 15000});
  } catch (error) {
    await captureOnFailure('duo-fold-draft-failed', error);
  } finally {
    writeFileSync(join(getArtifactsDir(), 'duo-fold-draft-postures.json'), JSON.stringify(evidence, null, 2));
    await restorePosture();
  }
}), 300000);

type TextRect = {x: number; y: number; width: number; height: number};
type NativeTextLayout = {
  revision: string; scale: number; selectionActive: boolean; queued: boolean;
  fontPoints: number[]; frame: TextRect; bounds: TextRect; measuredHeight: number;
  selectionRectCount: number; selectionBounds: TextRect; lastCharacterRect: TextRect;
};

async function verifyNativeTextLayout(panel: string, evidence: object[]) {
  const driver = getDriver();
  const container = execFileSync('xcrun', ['simctl', 'get_app_container', process.env.GTMUX_E2E_UDID!, 'com.gtmux.app', 'data'], {encoding: 'utf8'}).trim();
  const file = join(container, 'Documents', 'gtmux-text-layout.json');
  const views = await driver.$$('-ios predicate string:type == "XCUIElementTypeTextView" AND value != nil AND value != ""');
  const frames: TextRect[] = [];
  for (const view of views) {
    if (await view.isDisplayed()) frames.push(await driver.getElementRect(await view.elementId));
  }
  expect(frames.length).toBeGreaterThanOrEqual(3);
  let last = '', stable = 0;
  let matched: NativeTextLayout[] = [];
  const sameFrame = (a: TextRect, b: TextRect) => ['x', 'y', 'width', 'height'].every(key =>
    Math.abs(a[key as keyof TextRect] - b[key as keyof TextRect]) <= 2);
  try {
    await driver.waitUntil(() => {
      if (!existsSync(file)) return false;
      const records = Object.values(JSON.parse(readFileSync(file, 'utf8'))) as NativeTextLayout[];
      matched = frames.map(frame => records.find(record => sameFrame(record.frame, frame) && record.scale > 1))
        .filter((record): record is NativeTextLayout => !!record);
      if (matched.length !== frames.length) return false;
      const current = JSON.stringify(matched);
      stable = current === last ? stable + 1 : 0;
      last = current;
      return stable >= 2 && matched.every(record => !record.selectionActive && !record.queued &&
        !!record.revision && record.fontPoints.length > 0 && record.fontPoints.every(point => point > 0) &&
        record.selectionRectCount > 0 && record.measuredHeight <= record.bounds.height + 1 &&
        record.lastCharacterRect.height > 0 && record.lastCharacterRect.y >= record.bounds.y - 1 &&
        record.lastCharacterRect.y + record.lastCharacterRect.height <= record.bounds.y + record.bounds.height + 1 &&
        record.selectionBounds.y >= record.bounds.y - 1 &&
        record.selectionBounds.y + record.selectionBounds.height <= record.bounds.y + record.bounds.height + 1);
    }, {timeout: 20000, interval: 500, timeoutMsg: `${panel} native text did not settle into an unclipped current-scale layout`});
    evidence.push({panel, nativeTextLayouts: matched});
  } finally {
    if (existsSync(file)) writeFileSync(join(getArtifactsDir(), `duo-text-layout-${panel}.json`), readFileSync(file));
  }
}

it('keeps the session entry usable at an accessibility text size on both panels', async () => {
  const driver = getDriver();
  const evidence: object[] = [];
  let original: string | undefined;
  let auditFake: Fake | undefined;
  const auditErrors: string[] = [];
  const audit = async (panel: string) => {
    try {
      const issues = await driver.execute('mobile: performAccessibilityAudit');
      if (!Array.isArray(issues)) throw new Error('accessibility audit did not return an array');
      writeFileSync(join(getArtifactsDir(), `duo-a11y-${panel}.json`), JSON.stringify(issues, null, 2));
    } catch (error) {
      const message = `${panel}: ${String(error)}`;
      auditErrors.push(message);
      writeFileSync(join(getArtifactsDir(), `duo-a11y-${panel}-error.txt`), message);
    }
  };
  try {
    auditFake = await startFake();
    original = await driver.execute('mobile: contentSize') as string;
    expect(original).not.toMatch(/^(unknown|unsupported)$/);
    await driver.execute('mobile: setContentSize', {size: 'large'});
    expect(await driver.execute('mobile: contentSize')).toBe('large');
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: auditFake.url, GTMUX_DEBUG_PAIR_TOKEN: auditFake.token,
      GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en', GTMUX_DEBUG_TEXT_LAYOUT: '1'});
    await posture(0, 'outer', evidence);
    const back = driver.$(`~${TestIds.detail.back}`);
    if (await back.isDisplayed().catch(() => false)) await back.click();
    const row = driver.$('~agent-row-%13');
    await row.waitForDisplayed({timeout: 25000});
    await row.click();
    const selected = driver.$('~Show full title, 整理这周的发布清单');
    await selected.waitForDisplayed({timeout: 15000});
    // Exercise RN's live font-scale propagation into the native rich-text view;
    // a successful audit invocation alone does not prove that text scales.
    const message = '发版后本地也更新一下';
    const text = driver.$(`-ios predicate string:type == "XCUIElementTypeTextView" AND value == "${message}"`);
    await text.waitForDisplayed({timeout: 15000});
    const normalText = await text.getSize();
    await screenshot('duo-text-normal');
    await driver.execute('mobile: setContentSize', {size: 'accessibility-large'});
    expect(await driver.execute('mobile: contentSize')).toBe('accessibility-large');
    await driver.waitUntil(async () => (await text.getSize()).height > normalText.height,
      {timeout: 20000, interval: 500, timeoutMsg: 'native conversation text did not grow after live font-size change'});
    expect(await text.getValue()).toBe(message);
    evidence.push({textScaling: {normal: normalText, large: await text.getSize(), textPreserved: true}});
    const tools = driver.$('~session-tools');
    await tools.waitForDisplayed({timeout: 10000});
    await tools.click();
    await driver.$('~session-tools-menu-trace').waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-outer-menu');
    await driver.$('~Close').click();
    await tools.waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-outer');
    await verifyNativeTextLayout('outer', evidence);
    await audit('outer');
    await posture(180, 'inner', evidence);
    await selected.waitForDisplayed({timeout: 15000});
    await tools.waitForDisplayed({timeout: 10000});
    await tools.click();
    await driver.$('~session-tools-menu-trace').waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-inner-menu');
    await driver.$('~Close').click();
    await tools.waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-inner');
    await verifyNativeTextLayout('inner', evidence);
    await audit('inner');
    if (auditErrors.length) throw new Error(`Duo accessibility audit failed: ${auditErrors.join('; ')}`);
  } catch (error) {
    await captureOnFailure('duo-large-text-failed', error);
  } finally {
    writeFileSync(join(getArtifactsDir(), 'duo-large-text-postures.json'), JSON.stringify(evidence, null, 2));
    if (original && !/^(unknown|unsupported)$/.test(original)) {
      await driver.execute('mobile: setContentSize', {size: original}).catch(() => {});
    }
    await driver.execute('mobile: setSimulatedHingeAngle', {angle: 180}).catch(() => {});
    await driver.updateSettings({currentDisplayId: null}).catch(() => {});
    await auditFake?.close();
  }
}, 300000);

it('keeps Trace search and Diff open while the simulator folds and unfolds', async () => {
  const driver = getDriver();
  const evidence: object[] = [];
  try {
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
    await posture(180, 'inner', evidence);
    const row = driver.$('~agent-row-%13');
    await row.waitForDisplayed({timeout: 25000});
    await row.click();
    const tools = driver.$('~session-tools');
    await tools.waitForDisplayed({timeout: 10000});
    await tools.click();
    const traceAction = driver.$('~session-tools-menu-trace');
    await traceAction.waitForDisplayed({timeout: 10000});
    await traceAction.click();
    const title = driver.$('~trace-session-title');
    await title.waitForDisplayed({timeout: 15000});
    expect(await title.getText()).toContain('整理这周的发布清单');
    await driver.$('~trace-search-toggle').click();
    const search = driver.$('~trace-search');
    await search.waitForDisplayed({timeout: 10000});
    await typeInto('trace-search', 'part 12');
    expect(await search.getValue()).toBe('part 12');
    await posture(0, 'outer', evidence, '~trace-session-title');
    expect(await driver.$('~trace-search').getValue()).toBe('part 12');
    expect(await driver.$('~trace-session-title').getText()).toContain('整理这周的发布清单');
    await screenshot('duo-fold-trace-outer');
    await posture(180, 'inner', evidence, '~trace-session-title');
    expect(await driver.$('~trace-search').getValue()).toBe('part 12');
    await driver.$('~trace-close').click();
    await tools.waitForDisplayed({timeout: 10000});
    await tools.click();
    const diffAction = driver.$('~session-tools-menu-diff');
    await diffAction.waitForDisplayed({timeout: 10000});
    await diffAction.click();
    const wrap = driver.$('~diff-wrap');
    await wrap.waitForDisplayed({timeout: 15000});
    await posture(0, 'outer', evidence, '~diff-wrap');
    expect(await driver.$('~diff-wrap').isDisplayed()).toBe(true);
    await screenshot('duo-fold-diff-outer');
    await posture(180, 'inner', evidence, '~diff-wrap');
    expect(await driver.$('~diff-wrap').isDisplayed()).toBe(true);
    await driver.$('~Close changes').click();
    await driver.$('~session-tools').waitForDisplayed({timeout: 10000});
  } catch (error) {
    await captureOnFailure('duo-fold-tools-failed', error);
  } finally {
    writeFileSync(join(getArtifactsDir(), 'duo-fold-tools-postures.json'), JSON.stringify(evidence, null, 2));
    await driver.execute('mobile: setSimulatedHingeAngle', {angle: 180}).catch(() => {});
    await driver.updateSettings({currentDisplayId: null}).catch(() => {});
  }
}, 300000);
