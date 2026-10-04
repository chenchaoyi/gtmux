import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// The composer's Return key under Settings › "Return sends". Off (the default), Return
// makes a new line and only the send button sends. On, Return sends. Measured on
// 2026-10-05: with the setting on, the keyboard showed the blue send key and Return still
// only broke the line; nothing reached the Mac.
//
// Types with the on-screen keyboard (GTMUX_E2E_SOFT_KEYBOARD=1), and leaves the setting
// off afterwards.
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await setReturnSends(false).catch(() => {});
  await fake?.close();
});

const flags = () => ({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
const sends = () => fake.world.writesTo('/api/send') as Array<{text?: string; enter?: boolean}>;
// An empty text view reads back as no value, or as its placeholder.
const EMPTY = [null, '', 'Type a message…'];

async function launch() {
  const driver = getDriver();
  await launchWithFlags(flags());
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(1500);
}

async function setReturnSends(on: boolean) {
  const driver = getDriver();
  await launch();
  await driver.$(`~${TestIds.radar.settings}`).click();
  await settle(1500);
  // The row's Switch has no label of its own: take the one level with the row's title.
  for (let i = 0; i < 4; i++) {
    const title = driver.$('-ios predicate string:label BEGINSWITH "Return sends" AND visible == 1');
    if (await title.isExisting()) {
      const y = (await title.getLocation()).y;
      let sw: WebdriverIO.Element | undefined;
      let best = Infinity;
      for (const c of await driver.$$('-ios predicate string:type == "XCUIElementTypeSwitch" AND visible == 1').getElements()) {
        const d = Math.abs((await c.getLocation()).y - y);
        if (d < best) {
          best = d;
          sw = c;
        }
      }
      if (!sw) throw new Error('no switch beside "Return sends"');
      if (((await sw.getAttribute('value')) === '1') !== on) {
        await sw.click();
        await settle(800);
      }
      expect(await sw.getAttribute('value')).toBe(on ? '1' : '0');
      return;
    }
    await driver.execute('mobile: swipe', {direction: 'up'});
    await settle(600);
  }
  throw new Error('no "Return sends" row in Settings');
}

async function openComposer() {
  const driver = getDriver();
  await launch();
  await driver.$('~agent-row-%12').click();
  await settle(2500);
  await driver.$(`~${TestIds.composer.keyboard}`).click();
  const input = driver.$(`~${TestIds.composer.input}`);
  await input.waitForDisplayed({timeout: 8000});
  return input;
}

describe('composer: the Return key', () => {
  it('off: Return makes a new line, the send button sends', async () => {
    const driver = getDriver();
    await setReturnSends(false);
    const input = await openComposer();
    const before = sends().length;
    await input.addValue('first line');
    await input.addValue('\n');
    await input.addValue('second line');
    await settle(1000);
    await screenshot('return-off');
    expect(sends().length).toBe(before);
    expect(await input.getAttribute('value')).toBe('first line\nsecond line');
    await driver.$(`~${TestIds.composer.send}`).click();
    await settle(1500);
    expect(sends().length).toBe(before + 1);
    expect(sends()[sends().length - 1].text).toBe('first line\nsecond line');
  });

  it('on: Return sends', async () => {
    await setReturnSends(true);
    const input = await openComposer();
    const before = sends().length;
    await input.addValue('return sends this');
    await input.addValue('\n');
    await settle(2000);
    await screenshot('return-on');
    expect(sends().length).toBe(before + 1);
    expect(sends()[sends().length - 1].text).toBe('return sends this');
    // The field is empty again: no text left, and no new line typed after the send.
    expect(EMPTY).toContain(await input.getAttribute('value'));
    // And it keeps working: a second Return sends the next message too.
    await input.addValue('and this');
    await input.addValue('\n');
    await settle(2000);
    expect(sends().length).toBe(before + 2);
    expect(sends()[sends().length - 1].text).toBe('and this');
    expect(EMPTY).toContain(await input.getAttribute('value'));
  });
});
