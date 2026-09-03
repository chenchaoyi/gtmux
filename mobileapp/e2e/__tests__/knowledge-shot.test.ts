import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

// A look at the knowledge surface with REAL content (hq-knowledge-on-phone). The rules are
// pinned in knowledgeModel.test.ts / KnowledgeSheet.test.tsx; this is for reading the
// result with your eyes, which is the only way to judge whether it looks professional.
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('knowledge sheet', () => {
  it('opens from the header and drills into an entry', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
    });
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('kb-no-radar', err);
    }
    await settle(2500);
    await screenshot('kb-radar');
    await driver.$('~radar-hq-disc').click();
    await driver.$('~hq-verdict').waitForDisplayed({timeout: 10_000});
    await settle(1500);
    await driver.$('~hq-verdict').click(); // open the disclosure
    await settle(700);
    await screenshot('kb-0-header');

    const open = driver.$('~hq-knowledge-open');
    await open.waitForDisplayed({timeout: 8000});
    await open.click();
    await settle(1200);
    await screenshot('kb-1-index');

    // Into the first thing it owes the commander.
    const promo = driver.$('~knowledge-promotions');
    if (await promo.isExisting()) {
      const cards = await driver.$$('//*[starts-with(@name,"knowledge-promotion-")]');
      if (cards.length > 0) {
        await cards[0].click();
        await settle(1500);
        await screenshot('kb-2-entry');
        await driver.$('~knowledge-act-land').click();
        await settle(600);
        await screenshot('kb-3-act');
      }
    }
  });
});
