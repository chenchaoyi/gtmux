import {UsageReport} from '../api/client';
import {
  buildUsageView,
  compactTok,
  machineLines,
  planByAgent,
  rankSessions,
  sessionCount,
  unreadableReason,
} from './usageModel';

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
