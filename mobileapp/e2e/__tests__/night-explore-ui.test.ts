// Night exploration (maps, not verdicts): the composer's resting/open state across a
// relaunch, a long multi-line send and what reached the Mac, rotation, the largest text
// size, dark appearance, and a background round trip. Every step leaves a screenshot and
// the accessibility tree; the console lines are the findings.
import {execFileSync} from 'child_process';
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getDriver, getArtifactsDir} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

const UDID = process.env.GTMUX_E2E_UDID || 'booted';
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  try {
    execFileSync('xcrun', ['simctl', 'ui', UDID, 'content_size', 'large']);
    execFileSync('xcrun', ['simctl', 'ui', UDID, 'appearance', 'light']);
  } catch {
    /* best effort: restore the defaults */
  }
  await fake?.close();
});

async function dump(name: string) {
  await screenshot(`ui-${name}`);
  writeFileSync(join(getArtifactsDir(), `ui-${name}.xml`), await getDriver().getPageSource(), 'utf8');
}
const say = (...a: unknown[]) => {
  // eslint-disable-next-line no-console
  console.log('[ui]', ...a);
};
const shown = async (id: string) => getDriver().$(`~${id}`).isDisplayed().catch(() => false);

async function launchAndOpen(pane = '%12') {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1'});
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2000);
  await driver.$(`~agent-row-${pane}`).click();
  await settle(2500);
}

describe('night: UI map', () => {
  it('composer: resting vs open, and after a relaunch', async () => {
    const driver = getDriver();
    await launchAndOpen();
    say('launch1 before kbd: input shown =', await shown(TestIds.composer.input));
    await dump('c1-launch1-rest');
    await driver.$(`~${TestIds.composer.keyboard}`).click();
    await settle(1200);
    say('launch1 after kbd tap: input shown =', await shown(TestIds.composer.input));
    await dump('c2-launch1-open');
    await launchAndOpen();
    say('launch2 before kbd: input shown =', await shown(TestIds.composer.input));
    await dump('c3-launch2-rest');
    await driver.$(`~${TestIds.composer.keyboard}`).click();
    await settle(1200);
    say('launch2 after kbd tap: input shown =', await shown(TestIds.composer.input));
    await dump('c4-launch2-after-tap');
  });

  it('a long multi-line message reaches the Mac whole', async () => {
    const driver = getDriver();
    await launchAndOpen();
    if (!(await shown(TestIds.composer.input))) await driver.$(`~${TestIds.composer.keyboard}`).click();
    await settle(800);
    const long = Array.from({length: 30}, (_, i) => `第${i + 1}行：一段很长的说明文字，用来看输入框怎么换行和滚动 line ${i + 1}`).join('\n');
    await driver.$(`~${TestIds.composer.input}`).setValue(long);
    await settle(800);
    await dump('l1-typed-long');
    await driver.$(`~${TestIds.composer.send}`).click();
    await settle(2000);
    await dump('l2-after-send');
    const sends = fake.world.writesTo('/api/send') as Array<Record<string, unknown>>;
    const last = sends[sends.length - 1] ?? {};
    const text = String(last.text ?? '');
    say('long send: chars sent =', text.length, 'expected =', long.length, 'equal =', text === long);
  });

  it('rotation, largest text, dark, background round trip', async () => {
    const driver = getDriver();
    await launchAndOpen();
    await driver.setOrientation('LANDSCAPE');
    await settle(2000);
    await dump('r1-detail-landscape');
    await driver.$(`~${TestIds.detail.back}`).click().catch(() => {});
    await settle(1500);
    await dump('r2-radar-landscape');
    await driver.setOrientation('PORTRAIT');
    await settle(1500);
    execFileSync('xcrun', ['simctl', 'ui', UDID, 'content_size', 'accessibility-extra-extra-extra-large']);
    await settle(2500);
    await dump('a1-radar-ax-xxxl');
    await driver.$('~agent-row-%11').click().catch(() => {});
    await settle(2500);
    await dump('a2-waiting-detail-ax-xxxl');
    execFileSync('xcrun', ['simctl', 'ui', UDID, 'content_size', 'large']);
    execFileSync('xcrun', ['simctl', 'ui', UDID, 'appearance', 'dark']);
    await settle(2500);
    await dump('d1-dark');
    await driver.execute('mobile: backgroundApp', {seconds: 5});
    await settle(3000);
    await dump('b1-after-background');
    execFileSync('xcrun', ['simctl', 'ui', UDID, 'appearance', 'light']);
  });
});
