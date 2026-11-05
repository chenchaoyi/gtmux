import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

// A look at the HQ header's disclosure — the block rebuilt after 2026-09-03. Not an
// assertion: the separations are pinned in HQHeader.test.tsx; this is for reading the
// result with your eyes, which is the only way to judge whether it stopped looking
// scattered.
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('hq header', () => {
  it('opens the disclosure', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
    });
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('hdr-no-radar', err);
    }
    await settle(2000);
    await driver.$('~radar-hq-disc').click();
    await driver.$('~hq-verdict').waitForDisplayed({timeout: 10_000});
    await settle(2500); // let the transcript land so the brief has something to quote
    await screenshot('hqhdr-closed');
    await driver.$('~hq-verdict').click();
    await settle(600);
    await screenshot('hqhdr-open');
  });
});
