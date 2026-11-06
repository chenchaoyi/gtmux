import {execFileSync} from 'child_process';
import {existsSync, readFileSync, writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver} from '../setup/driver';
import {launchWithFlags} from '../setup/app';
import {captureOnFailure, screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';
import {TestIds} from '../../src/constants/testIds';
import {posture} from '../setup/duoPosture';
import {selectUnfoldedInnerDisplay} from '../setup/duoDisplay';
import {createJourneyGuard} from '../setup/journeyGuard';

const guard = createJourneyGuard();
beforeEach(selectUnfoldedInnerDisplay, 60000);
afterEach(guard.wait, 930000);

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

it('keeps the session entry usable at an accessibility text size on both panels', () => guard.run(async () => {
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
}), 300000);
