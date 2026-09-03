import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// Exploratory sweep against the fake, so every state the radar can show is on screen at
// once — including the ones a real machine rarely holds (an errored row, a plain pane, a
// native session, a waiting pane with real options). Screenshots for reading, not
// assertions: the behaviours have their own tests.
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

describe('every state, against the fake', () => {
  it('renders the radar, a waiting pane and the HQ page', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: fake.url,
      GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
    });
    const radar = driver.$(`~${TestIds.radar.screen}`);
    try {
      await radar.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('states-no-radar', err);
    }
    await settle(2500);
    await screenshot('states-1-radar');

    // The waiting row: its own words as buttons is the one case where the phone finishes
    // the job instead of routing you to the Mac.
    const rows = await driver.$$('//*[starts-with(@name,"agent-row-")]');
    // eslint-disable-next-line no-console
    console.log('[states] rows:', rows.length);
    if (rows.length > 0) {
      await rows[0].click();
      await settle(2000);
      await screenshot('states-2-detail');
      await driver.$(`~${TestIds.detail.back}`).click();
      await settle(800);
    }

    const disc = driver.$('~radar-hq-disc');
    if (await disc.isExisting()) {
      await disc.click();
      await settle(2500);
      await screenshot('states-3-hq');
      const verdict = driver.$('~hq-verdict');
      if (await verdict.isExisting()) {
        await verdict.click();
        await settle(700);
        await screenshot('states-4-hq-open');
      }
    }
  });
});
