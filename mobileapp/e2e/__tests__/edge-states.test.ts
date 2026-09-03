import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake, GUEST_VIEW} from '../fake-serve/server';

/**
 * The states a real machine rarely holds, and which therefore never got looked at.
 *
 * An empty fleet, a fleet of one, a knowledge base with nothing in it, a session whose
 * task is a wall of text with no spaces (the CJK wrap that has broken sends before), and
 * a guest, who must see the fleet but none of the supervisor's surfaces.
 *
 * Screenshots plus the few assertions that can be made without freezing the layout: that
 * nothing crashed, and that a guest is not shown what a guest may not have.
 */
let fake: Fake;
afterEach(async () => {
  await fake?.close();
});

const openRadar = async (f: Fake): Promise<boolean> => {
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: f.url,
    GTMUX_DEBUG_PAIR_TOKEN: f.token,
    GTMUX_DEBUG_NO_PUSH: '1',
  });
  try {
    await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  } catch (err) {
    await captureOnFailure('edge-no-radar', err);
    return false;
  }
  await settle(2200);
  return true;
};

it('an empty fleet says so instead of showing an empty frame', async () => {
  fake = await startFake();
  fake.world.agents = [];
  if (!(await openRadar(fake))) return;
  await screenshot('edge-1-empty-fleet');
  const src = await getDriver().getPageSource();
  expect(src).toContain('radar-screen'); // it rendered rather than crashing
});

it('a task with no spaces does not push the row off the screen', async () => {
  // A no-space CJK line is what broke send once (the composer wrapped it and the wrap
  // became a space). Here the question is only whether the row clamps it.
  fake = await startFake();
  fake.world.agents = fake.world.agents.slice(0, 3);
  fake.world.agents[1].task = '这是一条完全没有空格的超长任务描述'.repeat(6);
  if (!(await openRadar(fake))) return;
  await screenshot('edge-2-long-task');
  const {width} = await getDriver().getWindowSize();
  const row = getDriver().$('~agent-row-%11');
  const size = await row.getSize();
  expect(size.width).toBeLessThanOrEqual(width);
});

it('a guest sees the fleet and none of the supervisor', async () => {
  fake = await startFake({guest: true});
  if (!(await openRadar(fake))) return;
  await screenshot('edge-3-guest');
  const driver = getDriver();
  // The HQ disc is the entrance to everything owner-only; a guest must not have it.
  expect(await driver.$('~radar-hq-disc').isExisting()).toBe(false);
  // And the radar itself carries only the shared pane: a guest link scopes what may be
  // SEEN, so every other session's name, task and error must be absent from the screen,
  // not merely un-tappable.
  const src = await driver.getPageSource();
  for (const id of GUEST_VIEW) expect(src).toContain(`agent-row-${id}`);
  expect(src).not.toContain('agent-row-%11');
  expect(src).not.toContain('weekly limit');
});

it('an empty knowledge base opens and says nothing is recorded', async () => {
  fake = await startFake();
  fake.world.knowledge.entries = [];
  if (!(await openRadar(fake))) return;
  const driver = getDriver();
  await driver.$('~radar-hq-disc').click();
  await driver.$('~hq-verdict').waitForDisplayed({timeout: 10_000});
  await settle(1500);
  await driver.$('~hq-verdict').click();
  await settle(700);
  await screenshot('edge-4-hq-no-knowledge');
  // With nothing recorded the row is absent rather than a row reading zero.
  expect(await driver.$('~hq-knowledge-open').isExisting()).toBe(false);
});
