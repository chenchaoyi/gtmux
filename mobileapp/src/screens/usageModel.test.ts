import {UsageReport} from '../api/client';
import {buildUsageView, compactTok, machineLines, rankSessions, sessionCount} from './usageModel';

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
