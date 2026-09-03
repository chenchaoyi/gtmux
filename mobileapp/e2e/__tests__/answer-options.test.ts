import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

/**
 * Answering an agent's numbered question from the phone — the thing the commander does
 * on it most, and the one case where the phone finishes the job instead of routing you
 * to the Mac.
 *
 * The question after the tap is not "was a request made" but "did the session stop
 * asking". Those come apart: a digit delivered as a bracketed PASTE inserts the character
 * and selects nothing (the "tapping a number does nothing" regression, from when text
 * delivery moved to the paste buffer), and the screen would look identical until the next
 * poll. So this checks the payload shape AND what the radar says afterwards.
 */
let fake: Fake;
beforeEach(async () => {
  fake = await startFake();
});
afterEach(async () => {
  await fake?.close();
});

it('renders the agent’s own words, sends the digit, and the session stops waiting', async () => {
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1',
  });
  try {
    await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  } catch (err) {
    return captureOnFailure('ans-no-radar', err);
  }
  await settle(2200);

  await driver.$('~agent-row-%11').click();
  await settle(2500);
  await screenshot('ans-1-asking');

  // The agent's own words, not a generic "reply" affordance.
  const src = await driver.getPageSource();
  expect(src).toContain('可以,提到 red');

  // The card's own long-standing name for each choice.
  const opt = driver.$('~reply-1');
  await opt.waitForDisplayed({timeout: 8000});
  await opt.click();
  await settle(1500);
  await screenshot('ans-2-answered');

  // A digit is a KEYSTROKE: no enter, no paste. A menu commits on the keypress, and a
  // bracketed paste of "1" selects nothing.
  const sends = fake.world.writesTo('/api/send') as Array<Record<string, unknown>>;
  expect(sends.length).toBe(1);
  expect(sends[0].id).toBe('%11');
  expect(sends[0].text).toBe('1');
  expect(sends[0].enter).toBeFalsy();

  // And the mark clears: back on the radar the row is no longer in the waiting section.
  await driver.$(`~${TestIds.detail.back}`).click();
  await settle(2500);
  await screenshot('ans-3-radar-after');
  const after = await driver.getPageSource();
  expect(after).toContain('agent-row-%11');
  const waitingIdx = after.indexOf('radar-section-waiting');
  expect(waitingIdx).toBe(-1); // nothing is waiting any more
});
