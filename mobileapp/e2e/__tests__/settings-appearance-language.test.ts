import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// Settings › Appearance and Language: a choice applies at once and is still there after
// the app is relaunched. Both are put back to System afterwards.
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await launch().catch(() => {});
  await openSettings().catch(() => {});
  await pick(['Appearance', '外观'], 'system').catch(() => {});
  await pick(['Language', '语言'], 'system').catch(() => {});
  await fake?.close();
});

async function launch() {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1'});
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2000);
}

async function openSettings() {
  await getDriver().$(`~${TestIds.radar.settings}`).click();
  await settle(1500);
}

// On Settings: bring the row into view (its label starts with the title), open its picker,
// choose `key`.
async function pick(titles: string[], key: string) {
  const driver = getDriver();
  const q = titles.map(t => `label BEGINSWITH "${t}"`).join(' OR ');
  for (let i = 0; i < 4; i++) {
    const row = driver.$(`-ios predicate string:(${q}) AND visible == 1`);
    if (await row.isExisting()) {
      await row.click();
      await settle(1000);
      await driver.$(`~${TestIds.settings.pickerOption}-${key}`).click();
      await settle(1500);
      return;
    }
    await driver.execute('mobile: swipe', {direction: 'up'});
    await settle(700);
  }
  throw new Error(`no row ${titles.join('/')} in Settings`);
}

const shows = async (pattern: string) => getDriver().$(`-ios predicate string:label CONTAINS "${pattern}" AND visible == 1`).isExisting();

it('Dark and 中文 apply at once and survive a relaunch', async () => {
  await launch();
  await openSettings();
  await pick(['Appearance', '外观'], 'dark');
  await pick(['Language', '语言'], 'zh');
  await screenshot('settings-dark-zh');
  // At once: the page speaks Chinese, and the Appearance row says Dark in it.
  expect(await shows('外观')).toBe(true);
  expect(await shows('深色')).toBe(true);

  await launch();
  await openSettings();
  await screenshot('settings-after-relaunch');
  expect(await shows('外观')).toBe(true);
  expect(await shows('深色')).toBe(true);
});
