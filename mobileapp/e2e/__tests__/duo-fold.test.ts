import {writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver} from '../setup/driver';
import {launchWithFlags, readDebugLog, typeInto} from '../setup/app';
import {captureOnFailure, screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';
import {TestIds} from '../../src/constants/testIds';

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

it('retains the selected conversation, reading position and draft across simulator fold and unfold', async () => {
  const driver = getDriver();
  const evidence: object[] = [];
  try {
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
    await jump.click();
    await selected.waitForDisplayed({timeout: 15000});
    const keyboard = driver.$(`~${TestIds.composer.keyboard}`);
    await keyboard.waitForDisplayed({timeout: 10000});
    await keyboard.click();
    const input = driver.$(`~${TestIds.composer.input}`);
    await input.waitForDisplayed({timeout: 10000});
    await input.setValue('Duo fold draft');
    expect(await input.getValue()).toBe('Duo fold draft');
    expect(await driver.$('XCUIElementTypeKeyboard').isExisting()).toBe(true);
    const inner = await driver.$('~window-metrics-root').getSize();
    await screenshot('duo-fold-inner-draft');

    const outer = await posture(0, 'outer', evidence);
    expect([outer.width, outer.height]).not.toEqual([inner.width, inner.height]);
    await driver.$(`~${TestIds.composer.input}`).waitForDisplayed({timeout: 15000});
    expect(await driver.$(`~${TestIds.composer.input}`).getValue()).toBe('Duo fold draft');
    expect(await driver.$(`~${TestIds.detail.modeChat}`).isExisting()).toBe(true);
    expect(await selected.isDisplayed()).toBe(true);
    await screenshot('duo-fold-outer-draft');

    await posture(180, 'inner', evidence);
    await driver.$(`~${TestIds.composer.input}`).waitForDisplayed({timeout: 15000});
    expect(await driver.$(`~${TestIds.composer.input}`).getValue()).toBe('Duo fold draft');
    expect(await driver.$(`~${TestIds.detail.modeChat}`).isExisting()).toBe(true);
    expect(await selected.isDisplayed()).toBe(true);
    await screenshot('duo-fold-inner-restored');
  } catch (error) {
    await captureOnFailure('duo-fold-failed', error);
  } finally {
    writeFileSync(join(getArtifactsDir(), 'duo-fold-postures.json'), JSON.stringify(evidence, null, 2));
    writeFileSync(join(getArtifactsDir(), 'duo-fold-reflow-debug.json'), JSON.stringify(readDebugLog().filter(item => String(item.event).startsWith('chat-reflow-')), null, 2));
    await driver.execute('mobile: setSimulatedHingeAngle', {angle: 180}).catch(() => {});
    await driver.updateSettings({currentDisplayId: null}).catch(() => {});
  }
}, 300000);

it('keeps the session entry usable at an accessibility text size on both panels', async () => {
  const driver = getDriver();
  const evidence: object[] = [];
  let original: string | undefined;
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
    original = await driver.execute('mobile: contentSize') as string;
    expect(original).not.toMatch(/^(unknown|unsupported)$/);
    await driver.execute('mobile: setContentSize', {size: 'accessibility-large'});
    expect(await driver.execute('mobile: contentSize')).toBe('accessibility-large');
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
    await posture(0, 'outer', evidence);
    const back = driver.$(`~${TestIds.detail.back}`);
    if (await back.isDisplayed().catch(() => false)) await back.click();
    const row = driver.$('~agent-row-%13');
    await row.waitForDisplayed({timeout: 25000});
    await row.click();
    const selected = driver.$('~Show full title, 整理这周的发布清单');
    await selected.waitForDisplayed({timeout: 15000});
    const tools = driver.$('~session-tools');
    await tools.waitForDisplayed({timeout: 10000});
    await tools.click();
    await driver.$('~session-tools-menu-trace').waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-outer-menu');
    await driver.$('~Close').click();
    await tools.waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-outer');
    await audit('outer');
    await posture(180, 'inner', evidence);
    await selected.waitForDisplayed({timeout: 15000});
    await tools.waitForDisplayed({timeout: 10000});
    await tools.click();
    await driver.$('~session-tools-menu-trace').waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-inner-menu');
    await audit('inner');
    await driver.$('~Close').click();
    await tools.waitForDisplayed({timeout: 10000});
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
