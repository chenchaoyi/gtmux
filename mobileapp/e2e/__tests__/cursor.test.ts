import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

/**
 * Manual cursor-position check (NOT a regression assertion). Pairs to the in-process
 * fake, opens an agent's TERMINAL pane, and screenshots so we can eyeball where the cyan
 * cursor decoration lands vs the real input cursor — and confirm the terminal is NOT
 * black (the failure mode that #200 shipped to the device).
 *
 * Every tmux pane of the fake reports a cursor at the end of its last line
 * (world.seedCursor), hidden on the working pane the way an agent mid-turn hides it.
 * GTMUX_CURSOR is the switch: an eyeball check has nobody to read it in an unattended run.
 *
 *   GTMUX_CURSOR=1 GTMUX_CURSOR_TAG=baseline \
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e -- -t cursor
 */
const gated = process.env.GTMUX_CURSOR ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/cursor');
const TAG = process.env.GTMUX_CURSOR_TAG || 'shot';

gated('terminal cursor', () => {
  let fake: Fake;
  beforeAll(async () => {
    fake = await startFake();
    for (const a of fake.world.agents) {
      if (!a.pane_id.startsWith('%')) continue;
      fake.world.seedCursor(a.pane_id);
      if (a.status === 'working') fake.world.cursors.get(a.pane_id)!.visible = false;
    }
  });
  afterAll(async () => {
    await fake?.close();
  });

  it('opens a terminal pane and screenshots the cursor', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: fake.url,
      GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_PAIR_NAME: 'cursor',
      GTMUX_DEBUG_NO_PUSH: '1',
    });

    const radar = driver.$(`~${TestIds.radar.screen}`);
    await radar.waitForDisplayed({timeout: 25_000});
    // Open the agent row at GTMUX_CURSOR_IDX (default 0). Index 1+ is typically an
    // IDLE agent whose input cursor is VISIBLE — needed to eyeball placement (the
    // first/working agent hides its cursor).
    const idx = parseInt(process.env.GTMUX_CURSOR_IDX || '0', 10);
    const rows = await driver.$$(`-ios predicate string:name BEGINSWITH '${TestIds.agent.row}-'`);
    await settle(800);
    await rows[idx].click();
    await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 8_000});

    // switch to the TERMINAL mode (native <Text> renderer)
    const term = driver.$(`~${TestIds.detail.modeTerminal}`);
    await term.waitForDisplayed({timeout: 8_000});
    await term.click();
    await settle(3500); // let the terminal pane render + cursor place

    execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${TAG}.png`)], {stdio: 'ignore'});
    // eslint-disable-next-line no-console
    console.log(`[cursor] wrote ${join(OUT, `${TAG}.png`)}`);
  });
});
