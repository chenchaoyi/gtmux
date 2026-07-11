import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, openFirstAgentDetail, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Composer capture — drives Detail → reveal the input row → open the branded
 * attach sheet, saving PNGs for a visual review of the composer action row
 * (`+ · input · ⤢ · ↑` spacing + the ExpandIcon) and the AttachSheet. Gated on
 * GTMUX_SHOTS (+ a live serve), like screenshots.test.ts.
 */
const on = process.env.GTMUX_SHOTS && process.env.GTMUX_E2E_URL && process.env.GTMUX_E2E_TOKEN;
const gated = on ? describe : describe.skip;

const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/shots');

function shot(name: string): void {
  execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${name}.png`)], {stdio: 'ignore'});
}

gated('composer screenshots', () => {
  it('captures the composer input row + the attach sheet', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: process.env.GTMUX_E2E_URL!,
      GTMUX_DEBUG_PAIR_TOKEN: process.env.GTMUX_E2E_TOKEN!,
      GTMUX_DEBUG_NO_PUSH: '1',
    });

    if (!(await openFirstAgentDetail())) throw new Error('could not open an agent Detail');

    // Reveal the input row (tap the ⌨ key) → the keyboard rises and the row
    // `+ · input · ⤢ · ↑` docks above it.
    await driver.$(`~${TestIds.composer.keyboard}`).click();
    await settle(1200);
    shot('composer-input');

    // Open the branded attach sheet.
    await driver.$(`~${TestIds.composer.attach}`).click();
    await driver.$(`~${TestIds.composer.attachSheet}`).waitForDisplayed({timeout: 6_000});
    await settle(700);
    shot('composer-attach-sheet');

    // eslint-disable-next-line no-console
    console.log(`[shots] wrote composer-input + composer-attach-sheet to ${OUT}`);
  });
});
