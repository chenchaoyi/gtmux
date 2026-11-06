import {existsSync, readFileSync, writeFileSync} from 'fs';
import {execFileSync} from 'child_process';
import {join} from 'path';
import {getArtifactsDir, getDriver} from '../setup/driver';
import {launchWithFlags, readDebugLog, typeInto} from '../setup/app';
import {captureOnFailure, screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';
import {TestIds} from '../../src/constants/testIds';
import {createJourneyGuard} from '../setup/journeyGuard';
import {posture} from '../setup/duoPosture';

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

// A timed-out driver operation must finish cleanup before another case starts.
const guard = createJourneyGuard();
afterEach(guard.wait, 930000);

async function selectLongChat(evidence: object[]) {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en', GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_KEYBOARD: '1'});
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
    try {
      writeFileSync(join(getArtifactsDir(), 'duo-fold-draft-postures.json'), JSON.stringify(evidence, null, 2));
      const container = execFileSync('xcrun', ['simctl', 'get_app_container', process.env.GTMUX_E2E_UDID!, 'com.gtmux.app', 'data'], {encoding: 'utf8'}).trim();
      const file = join(container, 'Documents', 'gtmux-keyboard-evidence.json');
      if (existsSync(file)) writeFileSync(join(getArtifactsDir(), 'duo-fold-draft-keyboard.json'), readFileSync(file));
    } finally {
      await restorePosture();
    }
  }
}), 300000);


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
