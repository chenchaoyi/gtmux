import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

/**
 * What the commander types must arrive byte for byte.
 *
 * This machine's scar tissue is all here: a goal passed as a command-line argument got
 * eaten by the shell (hence `--goal-file`), a long CJK line with no spaces wrapped in the
 * composer and the wrap became a SPACE (which broke the tail match and mangled the
 * message), and backticks in prose have been executed before now.
 *
 * The phone's send is JSON over HTTP, so no shell should ever see it — that is the claim
 * this test makes checkable rather than assumed. It asserts on the exact string the server
 * received, which is the only place the truth is visible.
 */
let fake: Fake;
beforeEach(async () => {
  fake = await startFake();
});
afterEach(async () => {
  await fake?.close();
});

const HARD = [
  'a `backtick` and $(echo hi) and $HOME',
  "single 'quotes' and \"double\" and a \\backslash",
  '完全没有空格的中文长句子用来验证换行不会被当成空格插进去这句要足够长才有意义',
  'a line  with  doubled  spaces',
].join('\n');

it('a multi-line message with shell metacharacters arrives exactly as typed', async () => {
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1',
  });
  try {
    await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  } catch (err) {
    return captureOnFailure('fid-no-radar', err);
  }
  await settle(2200);
  await driver.$('~agent-row-%12').click(); // a working pane: it has a composer
  await settle(2500);

  // The composer rests as a row of key pills; the field appears when you tap ⌨ (MOBILE:
  // the resting row is what a reader sees, the field is what a writer asks for).
  await driver.$(`~${TestIds.composer.keyboard}`).click();
  const input = driver.$(`~${TestIds.composer.input}`);
  await input.waitForDisplayed({timeout: 8000});
  // setValue writes the whole string at once, newlines included — the point is the
  // payload, not the typing.
  await input.setValue(HARD);
  await settle(600);
  await screenshot('fid-1-typed');
  await driver.$(`~${TestIds.composer.send}`).click();
  await settle(1500);

  const sends = fake.world.writesTo('/api/send') as Array<Record<string, unknown>>;
  expect(sends.length).toBe(1);
  const got = String(sends[0].text ?? '');
  // Byte-exact, and reported as a diff rather than "false" when it is not.
  expect({text: got}).toEqual({text: HARD});
  // Newlines survived as newlines: the wrap-becomes-a-space bug would show up here.
  expect(got.split('\n').length).toBe(HARD.split('\n').length);
  // And it was submitted rather than left sitting in the box.
  expect(sends[0].enter).toBe(true);
});

it('trims what surrounds the message and nothing inside it', async () => {
  // The composer trims the whole message before sending (Composer.sendText). That is a
  // choice, not an accident — a stray space either side of a chat message means nothing to
  // an agent — so it is pinned here rather than left for someone to rediscover as a
  // fidelity bug. What must NOT happen is any change to what is between the ends.
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1',
  });
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2200);
  await driver.$('~agent-row-%12').click();
  await settle(2500);
  await driver.$(`~${TestIds.composer.keyboard}`).click();
  const input = driver.$(`~${TestIds.composer.input}`);
  await input.waitForDisplayed({timeout: 8000});
  await input.setValue('  keep   the   middle  ');
  await settle(500);
  await driver.$(`~${TestIds.composer.send}`).click();
  await settle(1500);
  const sends = fake.world.writesTo('/api/send') as Array<Record<string, unknown>>;
  expect(sends[0].text).toBe('keep   the   middle');
});

it('a second message into a working session does not displace the first', async () => {
  // hq's third item, in the part the API can actually answer. The agent queues the second
  // behind its current turn; what the phone must guarantee is that BOTH left, whole and in
  // order, with distinct idempotency tokens — a re-used token is how a retry is told from
  // a new message, and collapsing them would silently drop one.
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1',
  });
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2200);
  await driver.$('~agent-row-%12').click(); // working
  await settle(2500);
  await driver.$(`~${TestIds.composer.keyboard}`).click();
  const input = driver.$(`~${TestIds.composer.input}`);
  await input.waitForDisplayed({timeout: 8000});

  await input.setValue('第一条');
  await driver.$(`~${TestIds.composer.send}`).click();
  await settle(900);
  await input.setValue('第二条');
  await driver.$(`~${TestIds.composer.send}`).click();
  await settle(1500);
  await screenshot('fid-2-two-sends');

  const sends = fake.world.writesTo('/api/send') as Array<Record<string, unknown>>;
  expect(sends.map(s => s.text)).toEqual(['第一条', '第二条']);
  expect(new Set(sends.map(s => s.send_id)).size).toBe(2);
});
