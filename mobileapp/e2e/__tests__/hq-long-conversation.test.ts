import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// A long HQ conversation (31 turns of long paragraphs, code blocks and tables) on the HQ
// console's conversation tab: it opens on the latest turn; reading back offers the way
// down; the way down lands on the latest turn; and a reply that arrives while the reader
// is at the bottom shows up without a tap.
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

const ago = (mins: number) => new Date(Date.now() - mins * 60_000).toISOString();
const turn = (prompt: string, replies: string[], mins: number) => ({
  prompt,
  response: replies.join('\n\n'),
  segments: replies.map(text => ({text})),
  time: ago(mins),
});
const code = (n: number) => '```go\n' + Array.from({length: 40}, (_, i) => `func step${n}_${i}() error { return nil } // line ${i}`).join('\n') + '\n```';
const table = '| pane | state | note |\n|---|---|---|\n' + Array.from({length: 12}, (_, i) => `| %${i + 10} | working | 第${i}行，一段说明 |`).join('\n');
const list = Array.from({length: 15}, (_, i) => `- item ${i}: something to check`).join('\n');
const para = (n: number) => `第${n}轮：` + '这是一段很长的中文说明，用来看长段落怎么换行、滚动和折叠。'.repeat(12);

function conversation() {
  const turns = [];
  for (let n = 1; n <= 30; n++) {
    turns.push(turn(`» gtmux·tick #${n}`, [para(n), n % 3 === 0 ? code(n) : n % 3 === 1 ? table : list, `⟣ ▪ noted turn ${n}`], 120 - n * 3));
  }
  turns.push(turn('» gtmux·done %12', ['我去拉一下未读事件。', 'LATEST-TURN 最后一轮的结论在这里。', '⟣ ✅ 长对话 · 最后一条'], 1));
  return turns;
}

const seen = async (text: string) => getDriver().$(`-ios predicate string:label CONTAINS "${text}" AND visible == 1`).isExisting();
const jumpShown = async () => getDriver().$(`~${TestIds.detail.jumpBottom}`).isDisplayed().catch(() => false);

it('a long HQ conversation opens at the latest turn, reads back, and follows a new reply', async () => {
  const driver = getDriver();
  fake.world.transcripts.set('%6', conversation());
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'zh'});
  const disc = driver.$('~radar-hq-disc');
  await disc.waitForDisplayed({timeout: 25_000});
  await disc.click();
  await settle(3000);
  await driver.$('-ios predicate string:label BEGINSWITH "对话" AND visible == 1').click();
  await settle(6000);
  await screenshot('hq-long-open');
  expect(await seen('LATEST-TURN')).toBe(true);
  expect(await jumpShown()).toBe(false);

  for (let i = 0; i < 8; i++) {
    await driver.execute('mobile: swipe', {direction: 'down'});
    await settle(400);
  }
  await settle(1000);
  await screenshot('hq-long-read-back');
  expect(await seen('LATEST-TURN')).toBe(false);
  expect(await jumpShown()).toBe(true);

  await driver.$(`~${TestIds.detail.jumpBottom}`).click();
  await settle(2500);
  expect(await seen('LATEST-TURN')).toBe(true);
  expect(await jumpShown()).toBe(false);

  // A reply arrives the way a real one does: HQ works, then goes idle with the turn written.
  const hq = fake.world.agents.find(a => a.pane_id === '%6')!;
  hq.status = 'working';
  fake.bumpAgents();
  await settle(9000);
  fake.world.transcripts.set('%6', [...conversation(), turn('» gtmux·tick #new', ['NEW-TURN 新来的一轮', '⟣ ✅ 新的一条'], 0)]);
  hq.status = 'idle';
  fake.bumpAgents();
  await settle(9000);
  await screenshot('hq-long-new-reply');
  expect(await seen('NEW-TURN')).toBe(true);
});
