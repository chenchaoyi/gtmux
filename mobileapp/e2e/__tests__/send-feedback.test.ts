import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// What the phone says about a send it just made. Both cases need a server that refuses on
// command or a session that is mid-turn, which is why they were never driven before.
let fake: Fake;
beforeEach(async () => {
  fake = await startFake();
});
afterEach(async () => {
  await fake?.close();
});

const openComposer = async (pane: string) => {
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1',
  });
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2200);
  await driver.$(`~agent-row-${pane}`).click();
  await settle(2500);
  await driver.$(`~${TestIds.composer.keyboard}`).click();
  const input = driver.$(`~${TestIds.composer.input}`);
  await input.waitForDisplayed({timeout: 8000});
  return input;
};

it('a refusal names the pane someone is typing in', async () => {
  const driver = getDriver();
  fake.world.drafts.set('%12', 'half a thought'); // the core refuses this send
  let input;
  try {
    input = await openComposer('%12');
  } catch (err) {
    return captureOnFailure('sf-no-composer', err);
  }
  await input.setValue('继续');
  await driver.$(`~${TestIds.composer.send}`).click();
  await settle(1800);
  await screenshot('sf-1-draft-refusal');
  const src = await driver.getPageSource();
  expect(src).toContain('someone is typing in that pane');
  // The text is still held, so the retry is a promise the reader can check.
  expect(src).toContain('继续');
});

it('a send into a running session says it will be handled after this turn', async () => {
  const driver = getDriver();
  const input = await openComposer('%12'); // working in the fixtures
  await input.setValue('顺带看下这个');
  await driver.$(`~${TestIds.composer.send}`).click();
  await settle(1800);
  await screenshot('sf-2-busy-note');
  const src = await driver.getPageSource();
  expect(src).toContain('after the current turn');
});
