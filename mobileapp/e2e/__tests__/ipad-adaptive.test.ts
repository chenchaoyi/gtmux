import {writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver, recordSceneStep} from '../setup/driver';
import {launchWithFlags} from '../setup/app';
import {screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';
import {TestIds} from '../../src/constants/testIds';
import {captureSceneJourney, observeGeometry, readerToolsAndDraft, SceneJourney} from '../setup/sceneJourneys';

let fake: Fake;
beforeAll(async () => { fake = await startFake(); });
afterAll(async () => { await fake?.close(); });

it('keeps the iPad sidebar and selected session through rotation', async () => {
  const driver = getDriver();
  const original = await driver.getOrientation();
  const ctx: SceneJourney = {fake, surface: 'regular-iPad', steps: []};
  try {
    await driver.setOrientation('LANDSCAPE');
    const launchedAt = Date.now() / 1000;
    ctx.container = await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en', GTMUX_DEBUG_LOG_NET: '1'});
    const split = driver.$(`~${TestIds.radar.split}`);
    await split.waitForDisplayed({timeout: 25000});
    const before = await observeGeometry(ctx, launchedAt);
    expect(await driver.$('~agent-row-%13').isDisplayed()).toBe(true);
    recordSceneStep('sidebar-reader-landscape');
    await readerToolsAndDraft(ctx, before.native.geometry!);
    const identity = driver.$('-ios predicate string:label == "Show full title, 整理这周的发布清单"');
    expect(await identity.isDisplayed()).toBe(true);
    await screenshot('ipad-adaptive-landscape');
    const rotatedAt = Date.now() / 1000;
    await driver.setOrientation('PORTRAIT');
    const after = await observeGeometry(ctx, rotatedAt, before.native.geometry);
    expect(after.window).not.toEqual(before.window);
    writeFileSync(join(getArtifactsDir(), 'ipad-host-geometry.json'), JSON.stringify({before, after}, null, 2));
    expect(await split.isDisplayed()).toBe(true);
    expect(await driver.$(`~${TestIds.detail.modeChat}`).isExisting()).toBe(true);
    expect(await identity.isDisplayed()).toBe(true);
    expect(await driver.$(`~${TestIds.detail.back}`).isExisting()).toBe(false);
    expect(await driver.$('~agent-row-%13').isDisplayed()).toBe(true);
    recordSceneStep('sidebar-reader-portrait');
    expect(await driver.$('~composer-input').getValue()).toBe('scene migration draft');
    expect((await driver.$$('XCUIElementTypeKeyboard')).length).toBeGreaterThan(0);
    recordSceneStep('draft-keyboard-retained');
    await screenshot('ipad-adaptive-portrait');
  } finally {
    try {await captureSceneJourney(ctx, 'ipad');} finally {await driver.setOrientation(original);}
  }
});
