// The usage sheet, as DATA.
//
// It exists because the HQ header's redesign compressed three sensor rows into
// one, on the argument that the detail "already lives in the usage view" — and on
// the phone it did not. The radar reads `/api/usage` only to colour the HQ disc,
// so the header's USAGE row was the ONLY place a phone showed any of it. The
// argument was right about the header and wrong about the phone; this is the half
// that was missing.
//
// Its order follows `gtmux usage`, so the CLI and the phone answer the same
// question the same way round:
//
//   1. the PLAN — the one number local estimation cannot give you
//   2. who is BURNING it — per agent, then the sessions themselves
//   3. the MACHINE — the thing that stops all of it at once
//
// Sessions are ranked by trouble, not by size: a warned session is what you opened
// this sheet for, and a big idle one is just history.

import {ResourceReport, UsageHistory, UsageReport, UsageWindow} from '../api/client';

export interface AgentTotal {
  agent: string;
  sessions: number;
  tok: number;
  rate: number;
  warn?: string;
}

export interface SessionRow {
  paneId: string;
  loc: string;
  agent: string;
  tok: number;
  /** Live context as a fraction, 0 when the log carried none. */
  ctx: number;
  rate: number;
  warn?: string;
}

export interface UsageView {
  windows: UsageWindow[];
  totals: AgentTotal[];
  sessions: SessionRow[];
  machine: ResourceReport['machine'] | null;
}

/**
 * rankSessions puts the sessions in the order a reader needs them: anything the
 * core has warned about first, then by how fast it is burning, then by how full
 * its context is. Ties break on the pane id so two reads of an unchanged fleet are
 * identical rather than reshuffled.
 */
export function rankSessions(rows: SessionRow[]): SessionRow[] {
  return [...rows].sort((a, b) => {
    const w = Number(!!b.warn) - Number(!!a.warn);
    if (w) return w;
    if (b.rate !== a.rate) return b.rate - a.rate;
    if (b.ctx !== a.ctx) return b.ctx - a.ctx;
    return a.paneId.localeCompare(b.paneId);
  });
}

/** buildUsageView turns the endpoint's report into what the sheet renders. */
export function buildUsageView(u: UsageReport | null): UsageView {
  const sessions: SessionRow[] = (u?.sessions ?? []).map(s => ({
    paneId: s.pane_id ?? '',
    loc: s.loc ?? '',
    agent: s.agent ?? s.agent_key ?? '',
    tok: s.tok ?? 0,
    ctx: s.ctx ?? 0,
    rate: s.rate ?? 0,
    warn: s.usage_warn,
  }));
  return {
    windows: u?.limits?.windows ?? [],
    totals: (u?.types ?? []).map(t => ({
      agent: t.agent_key,
      sessions: t.sessions,
      tok: t.tok,
      rate: t.rate,
      warn: t.usage_warn,
    })),
    sessions: rankSessions(sessions),
    machine: u?.resource?.machine ?? null,
  };
}

/**
 * agentNames maps a registry key to the display name, learned from the session
 * rows rather than a table: each carries both, so "claude" becomes "Claude Code"
 * without this file having to know that. An unknown key keeps its own spelling —
 * inventing a capitalisation for an agent gtmux has not met would be worse.
 */
export function agentNames(u: UsageReport | null): Record<string, string> {
  const out: Record<string, string> = {};
  for (const s of u?.sessions ?? []) {
    const key = s.agent_key;
    const label = s.agent;
    if (key && label) out[key] = label;
  }
  return out;
}

/** One agent's plan: the windows that belong to it, named without its prefix. */
export interface PlanGroup {
  agent: string;
  name: string;
  windows: {name: string; kind?: string; pct: number; resetAt: string; resetUnix?: number}[];
  /**
   * Set when this agent's plan could not be read at all, carrying the Mac's reason key.
   *
   * The group still appears, with its name and icon and no numbers. An agent that
   * reports through its own session log goes quiet the moment it is not used — the last
   * reading's windows roll over and every row for it vanishes, which reads as the app
   * having broken. A group that says why is the difference.
   */
  unreadable?: string;
}

/** The sentence for a reason key. Unknown keys still say something: silence is the bug. */
export function unreadableReason(reason: string, name: string, zh: boolean): string {
  if (reason === 'rolled-over') {
    return zh
      ? `上次报告的窗口已经过去。${name} 把额度写在会话日志里，跑一轮就能重新读到。`
      : `The window it last reported has ended. ${name} writes its plan into a session log, so one turn brings the figure back.`;
  }
  return zh ? '现在读不到额度。' : 'Its plan is not readable right now.';
}

/**
 * planByAgent groups the windows under the agent whose plan they are.
 *
 * The flat list repeated the agent on every row — "claude session", "claude week
 * (all models)", "claude week (fable)" — in the registry's lowercase key, next to
 * session rows that spell the same agent "Claude Code". Grouping says it once, in
 * the name the rest of the app uses, and lets each window be called what it
 * actually is.
 *
 * Order is the report's, not sorted: it arrives plan by plan already.
 */
export function planByAgent(u: UsageReport | null, zh = false): PlanGroup[] {
  const names = agentNames(u);
  const out: PlanGroup[] = [];
  const at = new Map<string, number>();
  for (const w of u?.limits?.windows ?? []) {
    const agent = w.agent ?? '';
    const prefix = agent ? agent + ' ' : '';
    const bare = prefix && w.label.startsWith(prefix) ? w.label.slice(prefix.length) : w.label;
    const name = windowName({kind: w.kind, model: w.model, label: bare}, zh);
    const key = agent || w.label;
    let i = at.get(key);
    if (i === undefined) {
      i = out.length;
      at.set(key, i);
      // The SERVER's spelling first: it reads the agent registry, which is where an
      // agent's name is declared. `names` is the fallback for an older serve, learned
      // from session rows — and it is exactly what fails for an agent that has a plan
      // and no live session (Codex read "codex" beside "Claude Code", 2026-09-10).
      out.push({agent, name: w.agent_name || names[agent] || agent, windows: []});
    }
    out[i].windows.push({name, kind: w.kind, pct: w.pct_used, resetAt: resetLabel(w.reset_at, w.reset_unix, zh), resetUnix: w.reset_unix});
  }
  // Agents the Mac could not read a plan for, appended as their own groups. Never
  // merged into one that has windows: an agent is either readable or it is not.
  for (const gap of u?.limits?.unknown ?? []) {
    if (at.has(gap.agent)) continue;
    out.push({
      agent: gap.agent,
      name: gap.agent_name || names[gap.agent] || gap.agent,
      windows: [],
      unreadable: gap.reason,
    });
  }
  return out;
}

/**
 * compactTok is the token count as a reader wants it: 2.9M, 830k, 412.
 *
 * Exact digits are noise at this magnitude — nobody acts on the difference between
 * 2,851,826 and 2.9M — and a column of full numbers is unreadable at a glance,
 * which is the only way this sheet is read.
 */
export function compactTok(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${Math.round(n / 1000)}k`;
  return String(n);
}

/** sessionCount reads as English or Chinese, and counts one session as one. */
export function sessionCount(n: number, zh: boolean): string {
  if (zh) return `${n} 个会话`;
  return `${n} session${n === 1 ? '' : 's'}`;
}

/** machineLines is the machine, one labelled fact per line, absent when unknown. */
export function machineLines(
  m: ResourceReport['machine'] | null,
  zh: boolean,
): {label: string; value: string; warn?: boolean}[] {
  if (!m) return [];
  const out: {label: string; value: string; warn?: boolean}[] = [];
  if (m.disk_free_gb != null) {
    out.push({
      label: zh ? '磁盘' : 'disk',
      value: `${m.disk_free_gb}GB ${zh ? '可用' : 'free'}${m.disk_use_pct != null ? ` · ${m.disk_use_pct}%` : ''}`,
      // Only the core's own tier decides the colour. A figure that looks low to a
      // reader is not a warning; the core is what knows where the line is.
      warn: m.tier === 'amber' || m.tier === 'red',
    });
  }
  if (m.mem_free_pct != null) {
    out.push({
      label: zh ? '内存' : 'memory',
      value: `${m.mem_free_pct}% ${zh ? '空闲' : 'free'}${m.mem_tier ? ` · ${m.mem_tier}` : ''}`,
    });
  }
  if (m.load_ratio != null && m.ncpu != null) {
    out.push({
      label: zh ? '负载' : 'load',
      value: `${m.load_ratio.toFixed(2)}× ${m.ncpu} ${zh ? '核' : 'cores'}`,
    });
  }
  return out;
}

// ── what the page leads with ────────────────────────────────────────────────
//
// The sheet opened with a section header and then a flat list. The two facts a reader
// came for — which window is tightest, and whether the machine is about to stop
// everything — sat at row 3 and row 25 respectively (measured 2026-09-10: 18 session
// rows pushed an amber disk warning off the bottom of the screen).
//
// So the page states them first, in a sentence, and the sections stay where they are.

export interface Tightest {
  /** The window's own name, with its agent prefix removed. */
  window: string;
  agent: string;
  agentName: string;
  pct: number;
  resetAt: string;
  resetUnix?: number;
}

/**
 * tightestWindow is the window closest to its limit.
 *
 * SESSION windows are excluded, the same exclusion `warnOf` makes on the Mac: a session
 * window resets within hours, so a high one is a normal working day, not news. When only
 * session windows exist the tightest of those is still better than nothing.
 */
export function tightestWindow(u: UsageReport | null, zh = false): Tightest | null {
  const all = planByAgent(u, zh).flatMap(g =>
    g.windows.map(w => ({
      window: w.name,
      kind: w.kind,
      agent: g.agent,
      agentName: g.name,
      pct: w.pct,
      resetAt: w.resetAt,
      resetUnix: w.resetUnix,
    })),
  );
  if (all.length === 0) return null;
  // By kind where the serve says it, by the printed word for an older serve.
  const weekly = all.filter(w => (w.kind ? w.kind !== 'session' : !/session|会话/i.test(w.window)));
  const pool = weekly.length > 0 ? weekly : all;
  return pool.reduce((a, b) => (b.pct > a.pct ? b : a));
}

/**
 * untilReset words the wait as a duration, from an epoch the report may or may not carry.
 *
 * "Sep 11 at 10:59pm" makes a reader do date arithmetic to answer "is that soon"; "1 天 7
 * 小时后" is the answer. Without `reset_unix` there is nothing to compute from and the
 * wall clock is returned unchanged — inventing a duration from a parsed English date
 * would be a guess about a timezone.
 */
export function untilReset(resetUnix: number | undefined, nowSecs: number, zh: boolean): string {
  if (!resetUnix || resetUnix <= nowSecs) return '';
  const mins = Math.round((resetUnix - nowSecs) / 60);
  if (mins < 60) return zh ? `${mins} 分钟后重置` : `resets in ${mins}m`;
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  if (h < 24) {
    if (m === 0) return zh ? `${h} 小时后重置` : `resets in ${h}h`;
    return zh ? `${h} 小时 ${m} 分钟后重置` : `resets in ${h}h ${m}m`;
  }
  const d = Math.floor(h / 24);
  const hr = h % 24;
  if (hr === 0) return zh ? `${d} 天后重置` : `resets in ${d}d`;
  return zh ? `${d} 天 ${hr} 小时后重置` : `resets in ${d}d ${hr}h`;
}

/**
 * splitSessions divides the list into the ones worth a row and the ones worth a count.
 *
 * A session that is warned or actually producing tokens is why the sheet was opened; one
 * sitting idle is history. Eighteen rows of history is a wall that hides both. The
 * warned ones are NEVER folded away — that is the whole reason `rankSessions` exists.
 */
export function splitSessions(rows: SessionRow[]): {shown: SessionRow[]; rest: SessionRow[]; restTok: number} {
  const shown = rows.filter(s => !!s.warn || s.rate > 0);
  const rest = rows.filter(s => !s.warn && s.rate <= 0);
  return {shown, rest, restTok: rest.reduce((n, s) => n + s.tok, 0)};
}

/**
 * machineWarn is the machine's own warning sentence, or "" when it has nothing to say.
 *
 * Worded here from `warn_key` and the readings, so the phone's language wins over the
 * serve's (a launchd serve has no language and said "disk getting low" under a Chinese
 * UI, 2026-09-15). An older serve sends no key: its own sentence is shown as before.
 */
export function machineWarn(m: ResourceReport['machine'] | null | undefined, zh = false): string {
  if (!m || (m.tier !== 'amber' && m.tier !== 'red')) return '';
  const gb = m.disk_free_gb ?? 0;
  const load = (m.load_ratio ?? 0).toFixed(1);
  const pct = m.battery?.percent ?? 0;
  switch (m.warn_key) {
    case 'disk-critical':
      return zh ? `磁盘告急 · 剩 ${gb}GB` : `disk critical · ${gb}GB free`;
    case 'disk-low':
      return zh ? `磁盘快满了 · 剩 ${gb}GB` : `disk getting low · ${gb}GB free`;
    case 'memory-critical':
      return zh ? '内存告急' : 'memory critical';
    case 'memory-warn':
      return zh ? '内存吃紧' : 'memory warn';
    case 'load-critical':
      return zh ? `负载告急 · ${load} 倍核数` : `load critical · ${load}×cores`;
    case 'load-high':
      return zh ? `负载偏高 · ${load} 倍核数` : `load high · ${load}×cores`;
    case 'battery-critical':
      return zh ? `电量告急 · ${pct}%` : `battery critical · ${pct}%`;
    case 'battery-low':
      return zh ? `电量偏低 · ${pct}%` : `battery low · ${pct}%`;
  }
  return m.warn ?? '';
}

/**
 * windowName words a window from its kind: "本周（全部模型）" for what the agent printed
 * as "week (all models)". English keeps the agent's own label, and so does a window
 * whose kind the serve did not send (older serve, or a label of no known shape).
 */
export function windowName(w: {kind?: string; model?: string; label: string}, zh: boolean): string {
  if (!zh || !w.kind) return w.label;
  switch (w.kind) {
    case 'hour':
      return '每小时';
    case 'session':
      return '会话';
    case 'day':
      return '每日';
    case 'week':
      return '本周';
    case 'month':
      return '本月';
    case 'week-all':
      return '本周（全部模型）';
    case 'week-model':
      return `本周（${w.model ?? ''}）`;
  }
  return w.label;
}

/**
 * resetLabel is the reset time as the reader would write it: the agent's own words in
 * English ("Sep 18 at 10:59pm"), a local date in Chinese when the epoch is known
 * ("9月18日 22:59"), and the printed words again when it is not.
 */
export function resetLabel(resetAt: string, resetUnix: number | undefined, zh: boolean): string {
  if (!zh || !resetUnix) return resetAt;
  const d = new Date(resetUnix * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  return `${d.getMonth() + 1}月${d.getDate()}日 ${hh}:${mm}`;
}

// MARK: tokens by day (usage-daily-totals)

/** One bar of the seven-day chart, ready to draw. */
export interface DayBar {
  date: string;
  /** Output tokens that day. */
  out: number;
  /** Bar height as a fraction of the tallest day, 0..1. */
  frac: number;
  /** Weekday initial in the reader's language ("M" / "一"). */
  weekday: string;
  today: boolean;
  /** Direct label on this bar: today's, and the tallest day's. */
  labelled: boolean;
}

export interface TokensView {
  today: number;
  week: number;
  bars: DayBar[];
  byAgent: {agent: string; name: string; today: number; week: number}[];
}

const weekdayEn = ['S', 'M', 'T', 'W', 'T', 'F', 'S'];
const weekdayZh = ['日', '一', '二', '三', '四', '五', '六'];

/**
 * tokensView turns `history` into the tokens block: two figures, seven bars, the week's
 * split per agent. Null when the serve carries no history (older than 1.0.23), so the
 * block is absent rather than a row of zeros.
 *
 * A single neutral series: colour is a status channel in this product, and one series
 * needs no legend — the title names it. Direct labels go on today and on the tallest
 * day only; a number on every bar is the anti-pattern the dataviz method names.
 */
export function tokensView(h: UsageHistory | null | undefined, zh: boolean): TokensView | null {
  if (!h || !h.days || h.days.length === 0) return null;
  const days = h.days;
  const max = days.reduce((m, d) => Math.max(m, d.out ?? 0), 0);
  const last = days[days.length - 1];
  const tallest = days.reduce((a, b) => ((b.out ?? 0) > (a.out ?? 0) ? b : a));
  const bars: DayBar[] = days.map(d => {
    const dt = new Date(d.date + 'T12:00:00');
    const wd = Number.isNaN(dt.getTime()) ? '' : (zh ? weekdayZh : weekdayEn)[dt.getDay()];
    const today = d.date === last.date;
    return {
      date: d.date,
      out: d.out ?? 0,
      frac: max > 0 ? (d.out ?? 0) / max : 0,
      weekday: wd,
      today,
      labelled: today || (d === tallest && (d.out ?? 0) > 0),
    };
  });
  return {
    today: h.today_out ?? last.out ?? 0,
    week: h.week_out ?? days.reduce((n, d) => n + (d.out ?? 0), 0),
    bars,
    byAgent: (h.by_agent ?? []).map(a => ({
      agent: a.agent_key,
      name: a.agent_name || a.agent_key,
      today: a.today_out ?? 0,
      week: a.week_out ?? 0,
    })),
  };
}
