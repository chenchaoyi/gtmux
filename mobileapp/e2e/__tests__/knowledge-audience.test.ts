import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';

/**
 * The knowledge sheet's audience exits, driven on the simulator in DEMO mode
 * (hq-knowledge-engine phase 5). The model and sheet tests pin the logic; this is the
 * eyes: does a pending `machine` entry offer "Write it in" with no text field, does an
 * `everyone` entry offer the issue link and "Withdraw", does the axes line render.
 * Gated on GTMUX_KB_SHOTS; screenshots land in .e2e-artifacts/kb/.
 *
 *   GTMUX_KB_SHOTS=1 npm run test:e2e -- knowledge-audience
 */
const on = process.env.GTMUX_KB_SHOTS;
const gated = on ? describe : describe.skip;

const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/kb');
const DEMO_LABELS = ['No Mac? See a demo', '没有 Mac？看看演示'];

function shot(name: string): void {
  execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${name}.png`)], {stdio: 'ignore'});
}

async function openDemoKnowledge(): Promise<void> {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_SHOT_MODE: '1'});
  let opened = false;
  for (const label of DEMO_LABELS) {
    const demo = driver.$(`~${label}`);
    if (await demo.isExisting().catch(() => false)) {
      await demo.click();
      opened = true;
      break;
    }
  }
  if (!opened) throw new Error('demo entry not found');
  const hqDisc = driver.$('~radar-hq-disc');
  await hqDisc.waitForDisplayed({timeout: 20_000});
  await settle(1200);
  await hqDisc.click();
  const door = driver.$('~hq-knowledge-open');
  await door.waitForDisplayed({timeout: 15_000});
  await door.click();
  await settle(1200);
}

gated('knowledge sheet: the exit an audience has', () => {
  it('shows the axes and offers Write it in for a machine promotion, without a text field', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await openDemoKnowledge();
    shot('01-index');

    const k2 = driver.$('~knowledge-entry-k2');
    await k2.waitForDisplayed({timeout: 10_000});
    await k2.click();
    const carry = driver.$('~knowledge-act-carry');
    await carry.waitForDisplayed({timeout: 10_000});
    await settle(600);
    shot('02-machine-entry');
    // The axes line is on screen.
    expect(await driver.$('~knowledge-axes').isDisplayed()).toBe(true);
    // No land-by-hand and no feedback on a machine promotion: gtmux carries it.
    expect(await driver.$('~knowledge-act-land').isExisting()).toBe(false);
    expect(await driver.$('~knowledge-act-feedback').isExisting()).toBe(false);
    expect(await driver.$('~knowledge-act-withdraw').isExisting()).toBe(true);

    await carry.click();
    await driver.$('~knowledge-act-bar').waitForDisplayed({timeout: 5_000});
    await settle(500);
    shot('03-carry-bar');
    // carry asks nothing but "now?": no input, and Confirm is enabled.
    expect(await driver.$('~knowledge-act-input').isExisting()).toBe(false);
    expect(await driver.$('~knowledge-act-submit').isEnabled()).toBe(true);
    await driver.$('~knowledge-act-cancel').click();
  });

  it('offers the issue link and withdraw for an everyone promotion', async () => {
    const driver = getDriver();
    await openDemoKnowledge();
    const k5 = driver.$('~knowledge-entry-k5');
    await k5.waitForDisplayed({timeout: 10_000});
    await k5.click();
    const feedback = driver.$('~knowledge-act-feedback');
    await feedback.waitForDisplayed({timeout: 10_000});
    await settle(600);
    shot('04-everyone-entry');
    expect(await driver.$('~knowledge-act-carry').isExisting()).toBe(false);
    expect(await driver.$('~knowledge-act-land').isExisting()).toBe(true);

    await driver.$('~knowledge-act-withdraw').click();
    await driver.$('~knowledge-act-input').waitForDisplayed({timeout: 5_000});
    await settle(400);
    shot('05-withdraw-bar');
    // withdraw asks why: the input is there and Confirm waits for it.
    expect(await driver.$('~knowledge-act-submit').isEnabled()).toBe(false);
  });
});
