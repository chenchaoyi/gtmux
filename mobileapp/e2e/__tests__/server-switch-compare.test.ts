// Identical test harness against an isolated main build and the candidate build.
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver, recordSceneStep} from '../setup/driver';
import {launchWithFlags, readDebugLog, settle} from '../setup/app';
import {screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';

let home: Fake;
let office: Fake;
beforeAll(async () => { home = await startFake(); office = await startFake(); });
afterAll(async () => { await home?.close(); await office?.close(); });

it('selects a server after a clean launch', async () => {
  const d = getDriver();
  const seed = JSON.stringify([{url: home.url, token: home.token, name: 'Home'}, {url: office.url, token: office.token, name: 'Office'}]);
  try {
    await launchWithFlags({GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_RESET_SERVERS: '1', GTMUX_DEBUG_SERVERS: seed});
    await settle(3000);
    const row = d.$('-ios predicate string:label BEGINSWITH "Home" AND visible == 1');
    await row.waitForDisplayed({timeout: 15000});
    await row.click();
    await settle(6000);
    const chip = d.$('~radar-server-chip');
    await chip.waitForDisplayed({timeout: 15000});
    expect(await chip.getAttribute('label')).toContain('Home');
    recordSceneStep('select-after');
  } finally {
    await screenshot('select-after');
    writeFileSync(join(getArtifactsDir(), 'select.xml'), await d.getPageSource(), 'utf8');
    writeFileSync(join(getArtifactsDir(), 'select-debug.json'), JSON.stringify(readDebugLog(), null, 2));
    writeFileSync(join(getArtifactsDir(), 'select-fake.json'), JSON.stringify({home: home.requests, office: office.requests}, null, 2));
  }
});

it('switches between two saved servers', async () => {
  const d = getDriver();
  const seed = JSON.stringify([{url: home.url, token: home.token, name: 'Home'}, {url: office.url, token: office.token, name: 'Office'}]);
  try {
    await launchWithFlags({GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_SERVERS: seed, GTMUX_DEBUG_SHOT_MODE: '1'});
    await d.$('~radar-server-chip').waitForDisplayed({timeout: 30000});
    await screenshot('switch-before');
    recordSceneStep('switch-before');
    await d.$('~radar-server-chip').click();
    const row = d.$('-ios predicate string:type == "XCUIElementTypeButton" AND label BEGINSWITH "Office,"');
    await row.waitForDisplayed({timeout: 15000});
    await row.click();
    await settle(6000);
    const chip = d.$('~radar-server-chip');
    await chip.waitForDisplayed({timeout: 15000});
    expect(await chip.getAttribute('label')).toContain('Office');
    recordSceneStep('switch-after');
  } finally {
    await screenshot('switch-after');
    writeFileSync(join(getArtifactsDir(), 'switch.xml'), await d.getPageSource(), 'utf8');
    writeFileSync(join(getArtifactsDir(), 'switch-debug.json'), JSON.stringify(readDebugLog(), null, 2));
    writeFileSync(join(getArtifactsDir(), 'switch-fake.json'), JSON.stringify({home: home.requests, office: office.requests}, null, 2));
  }
});
