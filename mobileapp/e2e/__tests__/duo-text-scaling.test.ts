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
import {hasContainedInk, NativeTextLayout, waitForNativeTextLayout} from '../setup/nativeTextLayout';

const guard = createJourneyGuard();
beforeEach(selectUnfoldedInnerDisplay, 60000);
afterEach(guard.wait, 930000);

async function verifyNativeTextLayout(container: string, panel: string, evidence: object[]) {
  const file = join(container, 'Documents', 'gtmux-text-layout.json');
  try {
    const result = await waitForNativeTextLayout(panel, () => existsSync(file)
      ? Object.values(JSON.parse(readFileSync(file, 'utf8'))) as NativeTextLayout[] : []);
    evidence.push(result);
    return result;
  } finally {
    if (existsSync(file)) writeFileSync(join(getArtifactsDir(), `duo-text-layout-${panel}.json`), readFileSync(file));
  }
}

it.each(['accessibility-large', 'accessibility-extra-extra-extra-large'])('keeps the session entry usable at an accessibility text size on both panels (%s)', size => guard.run(async () => {
  const driver = getDriver();
  const evidence: object[] = [];
  let original: string | undefined;
  let auditFake: Fake | undefined;
  let primaryError: unknown;
  const auditErrors: string[] = [];
  const currentScales = new Map<string, number>();
  let container: string;
  const suffix = size === 'accessibility-large' ? '' : '-maximum';
  const label = (panel: string) => panel + suffix;
  const verify = async (panel: string) => {
    try {
      const result = await verifyNativeTextLayout(container, label(panel), evidence);
      currentScales.set(panel, result.nativeTextLayouts[0].scale);
    }
    catch (error) {
      // Preserve the failure, but still collect both body audits for diagnosis.
      const message = `${panel}: ${String(error)}`;
      auditErrors.push(message);
      writeFileSync(join(getArtifactsDir(), `duo-text-layout-${label(panel)}-error.txt`), message);
    }
  };
  const audit = async (panel: string) => {
    try {
      const issues = await driver.execute('mobile: performAccessibilityAudit');
      if (!Array.isArray(issues)) throw new Error('accessibility audit did not return an array');
      writeFileSync(join(getArtifactsDir(), `duo-a11y-${label(panel)}.json`), JSON.stringify(issues, null, 2));
      const layouts = JSON.parse(readFileSync(join(getArtifactsDir(), `duo-text-layout-${label(panel)}.json`), 'utf8')) as Record<string, NativeTextLayout>;
      const links = issues.map((issue: any) => {
        const identifier = issue.elementAttributes?.rawIdentifier;
        const layout = layouts[identifier];
        return {identifier, auditType: issue.auditType, layout: layout ?? null,
          inkContained: !!layout && hasContainedInk(layout)};
      });
      writeFileSync(join(getArtifactsDir(), `duo-audit-ink-${label(panel)}.json`), JSON.stringify(links, null, 2));
      // Never erase audit warnings. Missing/stale/uncontained paint remains a
      // failing diagnostic, including an audited view outside the viewport.
      for (const link of links.filter(item => item.auditType === 'XCUIAccessibilityAuditTypeTextClipped')) {
        if (!link.inkContained || !link.layout || link.layout.scale !== currentScales.get(panel) || !link.layout.rendering.complete ||
            link.layout.rendering.utf16Coverage !== link.layout.utf16Length || link.layout.queued || link.layout.selectionActive) {
          throw new Error(`audited text ${link.identifier} lacks contained current ink`);
        }
      }
    } catch (error) {
      const message = `${panel}: ${String(error)}`;
      auditErrors.push(message);
      writeFileSync(join(getArtifactsDir(), `duo-a11y-${label(panel)}-error.txt`), message);
    }
  };
  try {
    auditFake = await startFake();
    original = await driver.execute('mobile: contentSize') as string;
    expect(original).not.toMatch(/^(unknown|unsupported)$/);
    await driver.execute('mobile: setContentSize', {size: 'large'});
    expect(await driver.execute('mobile: contentSize')).toBe('large');
    container = await launchWithFlags({GTMUX_DEBUG_PAIR_URL: auditFake.url, GTMUX_DEBUG_PAIR_TOKEN: auditFake.token,
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
    await screenshot('duo-text-normal' + suffix);
    await driver.execute('mobile: setContentSize', {size});
    expect(await driver.execute('mobile: contentSize')).toBe(size);
    await driver.waitUntil(async () => (await text.getSize()).height > normalText.height,
      {timeout: 20000, interval: 500, timeoutMsg: 'native conversation text did not grow after live font-size change'});
    expect(await text.getValue()).toBe(message);
    evidence.push({textScaling: {category: size, normal: normalText, large: await text.getSize(), textPreserved: true}});
    const tools = driver.$('~session-tools');
    await tools.waitForDisplayed({timeout: 10000});
    await tools.click();
    await driver.$('~session-tools-menu-trace').waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-outer-menu' + suffix);
    await driver.$('~Close').click();
    await tools.waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-outer' + suffix);
    await verify('outer');
    await audit('outer');
    await posture(180, 'inner', evidence);
    await selected.waitForDisplayed({timeout: 15000});
    await tools.waitForDisplayed({timeout: 10000});
    await tools.click();
    await driver.$('~session-tools-menu-trace').waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-inner-menu' + suffix);
    await driver.$('~Close').click();
    await tools.waitForDisplayed({timeout: 10000});
    await screenshot('duo-large-text-inner' + suffix);
    await verify('inner');
    await audit('inner');
    if (auditErrors.length) throw new Error(`Duo accessibility audit failed: ${auditErrors.join('; ')}`);
  } catch (error) {
    primaryError = error;
    await captureOnFailure('duo-large-text-failed' + suffix, error);
  } finally {
    try {
      writeFileSync(join(getArtifactsDir(), `duo-large-text-postures${suffix}.json`), JSON.stringify(evidence, null, 2));
    } catch (error) {
      if (!primaryError) throw error;
    } finally {
      if (original && !/^(unknown|unsupported)$/.test(original)) {
        await driver.execute('mobile: setContentSize', {size: original}).catch(() => {});
      }
      await driver.execute('mobile: setSimulatedHingeAngle', {angle: 180}).catch(() => {});
      await driver.updateSettings({currentDisplayId: null}).catch(() => {});
      await auditFake?.close();
    }
  }
}), 300000);
