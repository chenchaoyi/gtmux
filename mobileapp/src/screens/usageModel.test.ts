import {UsageReport} from '../api/client';
import {buildUsageView,
  compactTok,
  machineLines,
  planByAgent,
  rankSessions,
  sessionCount,
  unreadableReason, tightestWindow, untilReset, splitSessions, machineWarn, SessionRow, tokensView, windowName, resetLabel} from './usageModel';

// A real payload, trimmed, from the machine this was written on.
const report = {
  sessions: [
    {pane_id: '%7', loc: 'HSS AI Workspace:0.0', agent: 'Claude Code', agent_key: 'claude', tok: 2_851_826, ctx: 0.945, rate: 0, usage_warn: 'ctx 94%'},
    {pane_id: '%19', loc: 'gtmux dev:0.0', agent: 'Claude Code', agent_key: 'claude', tok: 8_300_000, ctx: 0.6, rate: 4_000},
    {pane_id: '%22', loc: 'skill review:0.0', agent: 'Codex', agent_key: 'codex', tok: 43_623, ctx: 0.35, rate: 0},
  ],
  types: [
    {agent_key: 'claude', sessions: 17, tok: 54_023_978, rate: 8_411},
    {agent_key: 'codex', sessions: 1, tok: 43_623, rate: 0},
  ],
  limits: {windows: [{label: 'claude week (all models)', pct_used: 27, reset_at: 'Sep 11 at 11pm', agent: 'claude'}]},
  resource: {machine: {disk_free_gb: 15, disk_use_pct: 97, mem_free_pct: 31, mem_tier: 'warn', load_ratio: 1.07, ncpu: 14, warn: 'disk getting low', tier: 'amber' as const}},
} as unknown as UsageReport;

describe('buildUsageView', () => {
  it('carries the plan, the per-agent totals, the sessions and the machine', () => {
    const v = buildUsageView(report);
    expect(v.windows).toHaveLength(1);
    expect(v.totals.map(t => t.agent)).toEqual(['claude', 'codex']);
    expect(v.sessions).toHaveLength(3);
    expect(v.machine?.disk_free_gb).toBe(15);
  });

  it('survives an empty report rather than throwing', () => {
    const v = buildUsageView(null);
    expect(v).toEqual({windows: [], totals: [], sessions: [], machine: null});
  });
});

describe('rankSessions', () => {
  it('puts what the core warned about first, however small it is', () => {
    // The warned session is what you opened the sheet for; a big idle one is history.
    const v = buildUsageView(report);
    expect(v.sessions[0].paneId).toBe('%7');
    expect(v.sessions[0].warn).toBe('ctx 94%');
    // Then by burn rate: %19 is running, %22 is not.
    expect(v.sessions.map(s => s.paneId)).toEqual(['%7', '%19', '%22']);
  });

  it('is stable, so two reads of an unchanged fleet look identical', () => {
    const rows = [
      {paneId: '%2', loc: 'b', agent: 'x', tok: 0, ctx: 0, rate: 0},
      {paneId: '%1', loc: 'a', agent: 'x', tok: 0, ctx: 0, rate: 0},
    ];
    expect(rankSessions(rows).map(r => r.paneId)).toEqual(['%1', '%2']);
    expect(rankSessions(rankSessions(rows)).map(r => r.paneId)).toEqual(['%1', '%2']);
  });
});

describe('compactTok', () => {
  it('reads at a glance, which is the only way this sheet is read', () => {
    expect(compactTok(2_851_826)).toBe('2.9M');
    expect(compactTok(43_623)).toBe('44k');
    expect(compactTok(412)).toBe('412');
  });
});

describe('machineLines', () => {
  it('is amber only on the core own tier, never on a figure that looks low', () => {
    const [disk, mem, load] = machineLines(buildUsageView(report).machine, false);
    expect(disk.value).toBe('15GB free · 97%');
    expect(disk.warn).toBe(true);
    expect(mem.warn).toBeUndefined(); // "warn" is the mem TIER, not the machine's
    expect(load.value).toBe('1.07× 14 cores');
  });

  it('omits what the core did not report, rather than printing a dash', () => {
    expect(machineLines({disk_free_gb: 40} as never, false)).toHaveLength(1);
    expect(machineLines(null, false)).toEqual([]);
  });
});

describe('sessionCount', () => {
  it('counts one session as one', () => {
    expect(sessionCount(1, false)).toBe('1 session');
    expect(sessionCount(17, false)).toBe('17 sessions');
    expect(sessionCount(1, true)).toBe('1 个会话');
  });
});

describe('planByAgent', () => {
  it('says the agent once, in the name the rest of the app uses', () => {
    // The flat list repeated a lowercase registry key on every row — "claude
    // session", "claude week (all models)" — beside session rows spelling the same
    // agent "Claude Code".
    const g = planByAgent({
      sessions: [{agent_key: 'claude', agent: 'Claude Code', tok: 0, rate: 0}],
      limits: {
        windows: [
          {label: 'claude session', pct_used: 31, reset_at: 'Sep 7', agent: 'claude'},
          {label: 'claude week (all models)', pct_used: 27, reset_at: 'Sep 11', agent: 'claude'},
          {label: 'codex week', pct_used: 1, reset_at: 'Sep 7', agent: 'codex'},
        ],
      },
    } as never);
    expect(g.map(x => x.name)).toEqual(['Claude Code', 'codex']);
    expect(g[0].windows.map(w => w.name)).toEqual(['session', 'week (all models)']);
    expect(g[1].windows[0].name).toBe('week');
  });

  it('keeps an unknown agent’s own spelling rather than inventing one', () => {
    const g = planByAgent({limits: {windows: [{label: 'zed week', pct_used: 3, reset_at: 'x', agent: 'zed'}]}} as never);
    expect(g[0].name).toBe('zed');
  });

  it('survives a serve that sends no agent field', () => {
    const g = planByAgent({limits: {windows: [{label: 'session', pct_used: 3, reset_at: 'x'}]}} as never);
    expect(g[0].windows[0].name).toBe('session');
  });
});

// An agent whose plan the Mac cannot read must still appear, saying why.
//
// Codex reports its plan passively — it writes the server's rate-limit response into a
// session log when it takes a turn — so a day without Codex is enough for the last
// reading's windows to roll over. Before this, every Codex row simply stopped appearing,
// which the operator read as the app having broken (2026-09-07).
describe('an unreadable plan', () => {
  const report = {
    sessions: [],
    limits: {
      windows: [{label: 'claude week (all models)', pct_used: 50, reset_at: 'Sep 11', agent: 'claude'}],
      unknown: [{agent: 'codex', reason: 'rolled-over'}],
    },
  } as never;

  it('gets its own group instead of vanishing', () => {
    const groups = planByAgent(report);
    expect(groups.map(g => g.agent)).toEqual(['claude', 'codex']);
    const codex = groups[1];
    expect(codex.windows).toHaveLength(0);
    expect(codex.unreadable).toBe('rolled-over');
  });

  it('says what happened and what brings the figure back', () => {
    const en = unreadableReason('rolled-over', 'Codex', false);
    expect(en).toContain('has ended');
    expect(en).toContain('one turn');
    expect(unreadableReason('rolled-over', 'Codex', true)).toContain('跑一轮');
  });

  it('still says something for a reason it does not recognise, because silence is the bug', () => {
    expect(unreadableReason('something-new', 'Codex', false)).toBeTruthy();
    expect(unreadableReason('something-new', 'Codex', true)).toBeTruthy();
  });

  it('never turns an agent that HAS windows into an unreadable one', () => {
    const both = {
      sessions: [],
      limits: {
        windows: [{label: 'codex week', pct_used: 3, reset_at: 'Sep 11', agent: 'codex'}],
        unknown: [{agent: 'codex', reason: 'rolled-over'}],
      },
    } as never;
    const groups = planByAgent(both);
    expect(groups).toHaveLength(1);
    expect(groups[0].unreadable).toBeUndefined();
  });
});

// The two facts a reader opens this sheet for, and where they come from.
describe('what the page leads with', () => {
  const win = (agent: string, label: string, pct: number, resetUnix?: number) =>
    ({agent, label, pct_used: pct, reset_at: 'Sep 11 at 10:59pm', reset_unix: resetUnix});
  const rep = (windows: unknown[]) => ({limits: {windows}} as never);

  it('names the weekly window closest to its limit', () => {
    const t = tightestWindow(rep([win('claude', 'claude week (all models)', 76), win('claude', 'claude week (fable)', 40)]));
    expect(t?.window).toBe('week (all models)');
    expect(t?.pct).toBe(76);
  });

  it('ignores session windows — a high one is a normal working day, not news', () => {
    const t = tightestWindow(rep([win('claude', 'claude session', 92), win('claude', 'claude week (all models)', 40)]));
    expect(t?.window).toBe('week (all models)');
  });

  it('falls back to a session window when that is all there is', () => {
    expect(tightestWindow(rep([win('claude', 'claude session', 92)]))?.pct).toBe(92);
  });

  it('has nothing to say when no plan was read', () => {
    expect(tightestWindow(rep([]))).toBeNull();
    expect(tightestWindow(null)).toBeNull();
  });

  it('carries the agent name the server supplied', () => {
    const t = tightestWindow({limits: {windows: [{agent: 'codex', agent_name: 'Codex', label: 'codex week', pct_used: 3, reset_at: 'x'}]}} as never);
    expect(t?.agentName).toBe('Codex');
  });
});

describe('untilReset', () => {
  const NOW = 1_789_000_000;
  it('answers "is that soon" instead of making the reader do date arithmetic', () => {
    expect(untilReset(NOW + 40 * 60, NOW, true)).toBe('40 分钟后重置');
    expect(untilReset(NOW + 2 * 3600 + 20 * 60, NOW, true)).toBe('2 小时 20 分钟后重置');
    expect(untilReset(NOW + 31 * 3600, NOW, true)).toBe('1 天 7 小时后重置');
    expect(untilReset(NOW + 31 * 3600, NOW, false)).toBe('resets in 1d 7h');
  });
  it('drops the zero remainder', () => {
    expect(untilReset(NOW + 3 * 3600, NOW, false)).toBe('resets in 3h');
    expect(untilReset(NOW + 48 * 3600, NOW, true)).toBe('2 天后重置');
  });
  it('says nothing rather than guessing when the report carries no epoch', () => {
    // The wall-clock string has no timezone; parsing it would be inventing one.
    expect(untilReset(undefined, NOW, true)).toBe('');
    expect(untilReset(NOW - 60, NOW, true)).toBe('');
  });
});

describe('splitSessions', () => {
  const s = (paneId: string, o: Partial<SessionRow> = {}): SessionRow =>
    ({paneId, loc: paneId, agent: 'claude', tok: 1000, ctx: 0, rate: 0, ...o});

  it('shows what is warned or actually producing, counts the rest', () => {
    const r = splitSessions([s('%1', {warn: 'ctx 99%'}), s('%2', {rate: 3000}), s('%3'), s('%4', {tok: 500})]);
    expect(r.shown.map(x => x.paneId)).toEqual(['%1', '%2']);
    expect(r.rest.map(x => x.paneId)).toEqual(['%3', '%4']);
    expect(r.restTok).toBe(1500);
  });

  it('never folds a warned session away, however quiet it is', () => {
    // The whole reason the list is ranked by trouble. A warned session with no rate is
    // the exact shape of the four that were sitting at ctx 87–99% on 2026-09-10.
    const r = splitSessions([s('%1', {warn: 'ctx 87%', rate: 0})]);
    expect(r.shown).toHaveLength(1);
    expect(r.rest).toHaveLength(0);
  });
});

describe('machineWarn', () => {
  it('speaks only when the core says amber or red', () => {
    expect(machineWarn({warn: 'disk getting low · 28GB free', tier: 'amber'})).toBe('disk getting low · 28GB free');
    expect(machineWarn({warn: 'disk getting low', tier: undefined})).toBe('');
    expect(machineWarn(null)).toBe('');
  });
});

describe('tokensView', () => {
  const history = {
    days: [
      {date: '2026-09-08', out: 4_000_000, in: 10},
      {date: '2026-09-09', out: 12_000_000, in: 10},
      {date: '2026-09-10', out: 9_000_000, in: 10},
      {date: '2026-09-11', out: 0, in: 0},
      {date: '2026-09-12', out: 20_000_000, in: 10},
      {date: '2026-09-13', out: 11_600_000, in: 10},
      {date: '2026-09-14', out: 12_400_000, in: 10},
    ],
    today_out: 12_400_000,
    week_out: 69_000_000,
    by_agent: [
      {agent_key: 'claude', agent_name: 'Claude Code', today_out: 12_000_000, week_out: 60_000_000},
      {agent_key: 'codex', agent_name: 'Codex', today_out: 400_000, week_out: 9_000_000},
    ],
  };

  it('is two figures, seven bars, and the split', () => {
    const v = tokensView(history, false)!;
    expect(v.today).toBe(12_400_000);
    expect(v.week).toBe(69_000_000);
    expect(v.bars).toHaveLength(7);
    expect(v.bars[6].today).toBe(true);
    expect(v.bars[4].frac).toBe(1);
    expect(v.bars[3].frac).toBe(0);
    expect(v.bars.map(b => b.labelled)).toEqual([false, false, false, false, true, false, true]);
    expect(v.bars.map(b => b.weekday).join('')).toBe('TWTFSSM');
    expect(tokensView(history, true)!.bars.map(b => b.weekday).join('')).toBe('二三四五六日一');
    expect(v.byAgent.map(a => `${a.name} ${a.week}`)).toEqual(['Claude Code 60000000', 'Codex 9000000']);
  });

  it('is absent without history, and never a row of zeros', () => {
    expect(tokensView(undefined, false)).toBeNull();
    expect(tokensView({days: []}, false)).toBeNull();
    // A flat week draws flat bars, and labels only today.
    const flat = tokensView({days: history.days.map(d => ({...d, out: 0}))}, false)!;
    expect(flat.bars.every(b => b.frac === 0)).toBe(true);
    expect(flat.bars.filter(b => b.labelled)).toHaveLength(1);
  });
});

// The serve names a window's identity as data (kind/model) and the reset as an epoch, so
// the phone words both in its own language instead of showing the agent's English
// (「week (all models) · Sep 18 at 11pm」 under a Chinese UI, 2026-09-15).
describe('words a window and its reset in the reader’s language', () => {
  const win = (kind: string, model?: string) => ({kind, model, label: 'week (all models)'});
  it('names the kind in Chinese and keeps the agent’s label in English', () => {
    expect(windowName(win('week-all'), true)).toBe('本周（全部模型）');
    expect(windowName(win('week-model', 'Fable'), true)).toBe('本周（Fable）');
    expect(windowName(win('session'), true)).toBe('会话');
    expect(windowName(win('week-all'), false)).toBe('week (all models)');
    // No kind (older serve, or a label of no known shape): the label is all there is.
    expect(windowName({label: 'something odd'}, true)).toBe('something odd');
  });
  it('writes the reset as a local date in Chinese only when the epoch is known', () => {
    const at = Math.floor(new Date(2026, 8, 18, 22, 59).getTime() / 1000);
    expect(resetLabel('Sep 18 at 10:59pm', at, true)).toBe('9月18日 22:59');
    expect(resetLabel('Sep 18 at 10:59pm', at, false)).toBe('Sep 18 at 10:59pm');
    expect(resetLabel('Sep 18 at 10:59pm', undefined, true)).toBe('Sep 18 at 10:59pm');
  });
  it('groups with the worded names, and picks the tightest by kind', () => {
    const u = {
      limits: {
        windows: [
          {label: 'claude session', pct_used: 90, reset_at: 'x', agent: 'claude', agent_name: 'Claude Code', kind: 'session'},
          {label: 'claude week (fable)', pct_used: 58, reset_at: 'Sep 18 at 10:59pm', agent: 'claude', agent_name: 'Claude Code', kind: 'week-model', model: 'Fable'},
        ],
      },
    } as never;
    expect(planByAgent(u, true)[0].windows.map(w => w.name)).toEqual(['会话', '本周（Fable）']);
    expect(tightestWindow(u, true)?.window).toBe('本周（Fable）');
    expect(tightestWindow(u, false)?.window).toBe('week (fable)');
  });
});

describe('the machine warning is worded from its key', () => {
  it('says the condition in Chinese from warn_key and the readings', () => {
    expect(machineWarn({tier: 'amber', warn: 'disk getting low · 45GB free', warn_key: 'disk-low', disk_free_gb: 45}, true)).toBe('磁盘快满了 · 剩 45GB');
    expect(machineWarn({tier: 'red', warn: 'battery critical · 8%', warn_key: 'battery-critical', battery: {percent: 8}}, true)).toBe('电量告急 · 8%');
    expect(machineWarn({tier: 'amber', warn: 'load high · 1.6×cores', warn_key: 'load-high', load_ratio: 1.62}, false)).toBe('load high · 1.6×cores');
  });
  it('shows an older serve’s own sentence, and nothing when the machine is fine', () => {
    expect(machineWarn({tier: 'amber', warn: 'disk getting low · 45GB free'}, true)).toBe('disk getting low · 45GB free');
    expect(machineWarn({warn: 'stale', warn_key: 'disk-low'}, true)).toBe('');
  });
});
