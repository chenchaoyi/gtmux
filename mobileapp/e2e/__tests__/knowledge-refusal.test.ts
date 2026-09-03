import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

/**
 * The knowledge surface when the server says no.
 *
 * Landing is the one action here whose refusal a reader can act on ("that one was already
 * landed"), and until the fake existed it could only be produced by putting the
 * commander's real ledger into that state. So the path shipped (#889) verified by hand on
 * the happy case only.
 */
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

describe('a refused knowledge action', () => {
  it('shows the server’s words and keeps what you typed', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: fake.url,
      GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1',
    });
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('kr-no-radar', err);
    }
    await settle(2000);
    await driver.$('~radar-hq-disc').click();
    await driver.$('~hq-verdict').waitForDisplayed({timeout: 10_000});
    await settle(1500);
    await driver.$('~hq-verdict').click();
    await settle(700);

    const open = driver.$('~hq-knowledge-open');
    await open.waitForDisplayed({timeout: 8000});
    await open.click();
    await settle(1200);

    // Rig the NEXT act to fail the way the core does, then drive a landing.
    fake.world.failNext.set('/api/hq/knowledge/act', {
      status: 400,
      error: 'best-practices/spawn-must-decide-model has no pending promotion to land (gtmux knowledge promotions)',
    });
    const cards = await driver.$$('//*[starts-with(@name,"knowledge-promotion-")]');
    expect(cards.length).toBeGreaterThan(0);
    await cards[0].click();
    await settle(1200);
    await driver.$('~knowledge-act-land').click();
    await settle(500);
    await driver.$('~knowledge-act-input').setValue('AGENTS.md');
    await driver.$('~knowledge-act-submit').click();
    await settle(900);
    await screenshot('kr-refused');

    const src = await driver.getPageSource();
    // The server's own sentence, not a generic failure.
    expect(src).toContain('no pending promotion to land');
    // And the draft survives, so the reader can correct rather than retype.
    const input = driver.$('~knowledge-act-input');
    expect(await input.getValue()).toBe('AGENTS.md');
  });
});
