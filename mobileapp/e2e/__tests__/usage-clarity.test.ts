import {execFileSync} from 'child_process';
import {UsageReport} from '../../src/api/client';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';

let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
  const at = Math.floor(Date.now() / 1000);
  const days = Array.from({length: 28}, (_, i) => {
    const d = new Date(); d.setDate(d.getDate() - 27 + i);
    const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
    const codex = 200_000 + i * 50_000;
    const dsh = i === 27 ? 29_000 : 10_000;
    return {date, out: codex + dsh, in: 0, by_agent: {codex: {out: codex, in: 0}, dsh: {out: dsh, in: 0}}};
  });
  const today = days[27]; const week = days.slice(-7);
  const usage: UsageReport = {
    sources: [{agent: 'dsh', name: 'DeepSeek Harness', installed: true, tokens: true, account: 'balance'}],
    limits: {at, dsh_account_status: 'signed-out', windows: [
      {agent: 'claude', agent_name: 'Claude Code', label: 'claude session', kind: 'session', pct_used: 0, reset_at: '', stale: true, observed_at: at - 24 * 86400},
      {agent: 'claude', agent_name: 'Claude Code', label: 'claude week (all models)', kind: 'week-all', pct_used: 1, reset_at: '', stale: true, observed_at: at - 24 * 86400},
      {agent: 'claude', agent_name: 'Claude Code', label: 'claude week (Fable)', kind: 'week-model', model: 'Fable', pct_used: 1, reset_at: '', stale: true, observed_at: at - 24 * 86400},
      {agent: 'codex', agent_name: 'Codex', label: 'codex week', kind: 'week', pct_used: 12, reset_at: '', reset_unix: at + 6 * 86400, observed_at: at - 120, plan_type: 'prolite'},
    ]},
    history: {days, today_out: today.out, week_out: week.reduce((n, d) => n + d.out, 0),
      by_agent: (['codex', 'dsh'] as const).map(key => ({agent_key: key, agent_name: key === 'codex' ? 'Codex' : 'DeepSeek Harness', today_out: today.by_agent[key].out, today_in: 0, week_out: week.reduce((n, d) => n + d.by_agent[key].out, 0), week_in: 0})),
      activity: {since: days[0].date, series: days.map(d => ({date: d.date, out: d.out})), all_out: days.reduce((n, d) => n + d.out, 0), peak_out: today.out, peak_date: today.date, streak: 28, best_streak: 28, active_days: 28, days_known: 28}},
  };
  fake.world.usage = usage;
});
afterAll(async () => { await fake?.close(); });

describe('usage capacity and output hierarchy', () => {
  for (const lang of ['en', 'zh']) {
    for (const appearance of ['light', 'dark']) {
      it(`keeps historical capacity inspectable and output separate (${lang}/${appearance})`, async () => {
        const driver = getDriver();
        execFileSync('xcrun', ['simctl', 'ui', process.env.GTMUX_E2E_UDID!, 'appearance', appearance]);
        await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
          GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: lang, GTMUX_DEBUG_RESET_SERVERS: '1'});
        await driver.$('~radar-screen').waitForDisplayed({timeout: 25_000});
        await driver.$('~radar-hq-disc').click();
        await driver.$('~hq-verdict').waitForDisplayed({timeout: 10_000});
        await settle(1800);
        await driver.$('~hq-verdict').click();
        await driver.$('~hq-usage-open').waitForDisplayed({timeout: 6000});
        await driver.$('~hq-usage-open').click();
        await driver.$('~usage-account-details').waitForDisplayed({timeout: 6000});
        await settle(700);
        expect(await driver.$('~usage-account-detail-content').isExisting()).toBe(false);
        await screenshot(`usage-${lang}-${appearance}-compact`);
        await driver.$('~usage-account-details').click();
        await driver.$('~usage-account-detail-content').waitForExist({timeout: 6000});
        expect(await driver.$('~usage-dsh-account-note').isExisting()).toBe(true);
        await screenshot(`usage-${lang}-${appearance}-expanded`);
        await driver.$('~usage-account-details').click();
        await driver.$('~usage-account-detail-content').waitForExist({reverse: true, timeout: 6000});
      });
    }
  }
});
