// The HQ header's status line (#1309): a turn holds every reply HQ made after the prompt
// that opened it, and the first reply is usually a working line, so the register line
// (`⟣ ✅/⚠/◈ …`) is rarely at the start of the turn's response. The header must find it in
// the turn's replies; a newer routine `⟣ ▪` line silences it; no `⟣` means nothing shows.
//
// Driven against the fake serve, whose HQ transcript a test sets per pane.
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getDriver, getArtifactsDir} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {startFake, Fake} from '../fake-serve/server';

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

async function openHQ(label: string): Promise<string> {
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1',
    GTMUX_DEBUG_LANG: 'zh',
  });
  const disc = driver.$('~radar-hq-disc');
  await disc.waitForDisplayed({timeout: 25_000});
  await disc.click();
  await driver.$('~hq-verdict').waitForDisplayed({timeout: 20_000});
  await settle(4000);
  // HQ's own words are inside the card's disclosure, which rests closed: open it.
  if (!(await driver.$('~hq-disclosure').isExisting())) {
    await driver.$('~hq-verdict').click();
    await settle(1500);
  }
  await screenshot(`hqsig-${label}`);
  const src = await driver.getPageSource();
  writeFileSync(join(getArtifactsDir(), `hqsig-${label}.xml`), src, 'utf8');
  // The brief's own node carries no text: its words follow it as siblings in the tree. So
  // "shown" is the brief node existing, and the words are read from the whole page (the
  // conversation tab is not open, so the reply text is not otherwise on screen).
  const brief = src.includes('name="hq-brief"');
  const words = (src.match(/label="([^"]*)"/g) ?? []).map(l => l.slice(7, -1));
  const text = brief ? [...new Set(words.filter(w => /^· (完成|升级|注意|\d)|已装好|等你拍板|水位/.test(w)))].join(' | ') : '(no hq-brief)';
  // eslint-disable-next-line no-console
  console.log(`[hqsig] ${label}: ${text}`);
  return text;
}

describe('HQ header status line (#1309)', () => {
  it('control: a reply that opens with the register line is shown (before and after)', async () => {
    fake.world.transcripts.set('%6', [turn('» gtmux·done %12', ['⟣ ✅ 1.0.83 已装好 · 菜单栏和命令行都是新版'], 5)]);
    const t = await openHQ('0-control-starts-with-signal');
    expect(t).toContain('1.0.83 已装好');
  });

  it('shows a register line that follows an opening line in the same turn', async () => {
    fake.world.transcripts.set('%6', [
      turn('» gtmux·done %12', ['我去拉一下未读事件。', '⟣ ✅ 1.0.83 已装好 · 菜单栏和命令行都是新版'], 6),
    ]);
    const t = await openHQ('1-opening-then-done');
    expect(t).toContain('1.0.83 已装好');
  });

  it('a newer routine ▪ line silences it', async () => {
    fake.world.transcripts.set('%6', [
      turn('» gtmux·done %12', ['我去拉一下未读事件。', '⟣ ✅ 1.0.83 已装好 · 菜单栏和命令行都是新版'], 9),
      turn('» gtmux·tick', ['先看一下水位。', '⟣ ▪ noted: 水位到 32134'], 2),
    ]);
    const t = await openHQ('2-routine-newer');
    expect(t).toBe('(no hq-brief)');
  });

  it('no register line, nothing shown', async () => {
    fake.world.transcripts.set('%6', [turn('» gtmux·tick', ['看了一下，没有新事件。'], 3)]);
    const t = await openHQ('3-no-signal');
    expect(t).toBe('(no hq-brief)');
  });

  it('a warning reply after an opening line shows as the escalation', async () => {
    fake.world.transcripts.set('%6', [turn('» gtmux·asks %11', ['我核对一下。', '⟣ ⚠ %11 等你拍板 · 红档要不要改'], 4)]);
    const t = await openHQ('4-opening-then-warning');
    expect(t).toContain('%11 等你拍板');
    expect(t).toContain('升级');
  });
});
