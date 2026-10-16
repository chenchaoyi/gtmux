// Synthetic accounting for the tour. Session identity comes from the same fleet as
// the radar; aggregate rows and daily totals are derived rather than copied.
import {UsageReport, UsageSession} from '../api/client';
import {Agent} from '../api/types';
import {humanReset} from './demoData';

export function demoUsage(fleet: Agent[]): UsageReport {
  const keys: Record<string, string> = {'Claude Code': 'claude', Codex: 'codex', Gemini: 'gemini'};
  const sessions: UsageSession[] = fleet.filter(a => a.source === 'tmux').map((a, i) => ({
    pane_id: a.pane_id, loc: a.loc, agent: a.agent, agent_key: keys[a.agent],
    tok: (i + 1) * 125_000, ctx: 0.2 + i * 0.07, rate: a.status === 'working' ? 1200 : 0,
  }));
  const types = [...new Set(sessions.map(s => s.agent_key))].map(agent_key => {
    const rows = sessions.filter(s => s.agent_key === agent_key);
    return {agent_key, sessions: rows.length, tok: rows.reduce((n, s) => n + s.tok, 0), rate: rows.reduce((n, s) => n + s.rate, 0)};
  });
  const days = Array.from({length: 7}, (_, i) => {
    const d = new Date(Date.now());
    d.setDate(d.getDate() - 6 + i);
    const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
    const claude = {out: 12_000 + i * 1800, in: 100_000 + i * 9000};
    const codex = {out: 8000 + i * 1000, in: 50_000 + i * 5000};
    return {date, out: claude.out + codex.out, in: claude.in + codex.in, by_agent: {claude, codex}};
  });
  const last = days[6];
  const week_out = days.reduce((n, d) => n + d.out, 0);
  const week_in = days.reduce((n, d) => n + d.in, 0);
  const at = (h: number) => humanReset(Date.now() + h * 3600e3);
  const unix = (h: number) => Math.floor(Date.now() / 1000 + h * 3600);
  return {
    sessions, types,
    history: {
      days, today_out: last.out, today_in: last.in, week_out, week_in,
      by_agent: (['claude', 'codex'] as const).map(agent_key => ({
        agent_key, agent_name: agent_key === 'claude' ? 'Claude Code' : 'Codex',
        today_out: last.by_agent[agent_key].out, today_in: last.by_agent[agent_key].in,
        week_out: days.reduce((n, d) => n + d.by_agent[agent_key].out, 0),
        week_in: days.reduce((n, d) => n + d.by_agent[agent_key].in, 0),
      })),
      scanned_at: Math.floor(Date.now() / 1000),
      activity: {since: days[0].date, series: days.map(({date, out}) => ({date, out})),
        all_out: week_out, peak_out: last.out, peak_date: last.date, streak: 7, best_streak: 7, active_days: 7, days_known: 7},
    },
    limits: {at: Math.floor(Date.now() / 1000) - 240, windows: [
      {label: 'claude session', pct_used: 62, reset_at: at(2.4), reset_unix: unix(2.4), agent: 'claude', agent_name: 'Claude Code'},
      {label: 'claude week (all models)', pct_used: 41, reset_at: at(84), reset_unix: unix(84), agent: 'claude', agent_name: 'Claude Code'},
      {label: 'codex week', pct_used: 18, reset_at: at(96), reset_unix: unix(96), agent: 'codex', agent_name: 'Codex'},
    ]},
    resource: {machine: {disk_free_gb: 84, mem_free_pct: 38, mem_tier: 'ok', warn: ''}},
  };
}
