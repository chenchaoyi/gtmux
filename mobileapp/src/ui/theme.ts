// Design tokens — mirrors macapp/.../Theme.swift. Status colors are the
// AUTHORITATIVE hex (DESIGN §1/§9); keep identical across all three surfaces.

import {Agent, SectionKey, StatusName, primary} from '../api/types';

export const StatusColor: Record<StatusName, string> = {
  waiting: '#EF4444', // red
  working: '#06B6D4', // cyan
  idle: '#22C55E', // green
  running: '#8E8E93', // gray (none / running)
};

// MODIFIER color (not a state): the errored-idle ⚠ marker. Amber, distinct from
// the state palette so it can't be mistaken for waiting (red). Mirrors Theme.swift.
export const ERRORED_COLOR = '#F59E0B'; // amber

// Section + sort order: needs-you → working → idle → running (DESIGN §3).
export const statusRank: Record<StatusName, number> = {
  waiting: 0,
  working: 1,
  idle: 2,
  running: 3,
};

export const SECTION_ORDER: StatusName[] = ['waiting', 'working', 'idle', 'running'];

export const Size = {
  avatar: 34,
  badge: 16,
  radiusAvatar: 9, // app-icon-style rounded square (MOBILE §2)
  radiusRow: 12,
  radiusBadgeSquare: 4,
  pad: 14,
  gap: 12,
};

// Light + dark palettes resolved by useColorScheme (like Theme.Palette.of).
export interface Palette {
  bg: string;
  surface: string;
  fg: string;
  fg2: string;
  fg3: string;
  divider: string;
  divLoud: string; // the 3px section-separator line (MOBILE §3)
  rowSelected: string;
  waitingTint: string;
}

const dark: Palette = {
  bg: '#0D0D0F',
  surface: '#1C1C1F',
  fg: 'rgba(255,255,255,0.96)',
  fg2: 'rgba(235,235,245,0.62)',
  fg3: 'rgba(235,235,245,0.34)',
  divider: 'rgba(255,255,255,0.09)',
  divLoud: 'rgba(255,255,255,0.16)',
  rowSelected: 'rgba(255,255,255,0.06)',
  waitingTint: 'rgba(239,68,68,0.10)',
};

const light: Palette = {
  bg: '#F2F2F7',
  surface: '#FFFFFF',
  fg: '#1D1D1F',
  fg2: 'rgba(60,60,67,0.62)',
  fg3: 'rgba(60,60,67,0.34)',
  divider: 'rgba(0,0,0,0.08)',
  divLoud: 'rgba(0,0,0,0.16)',
  rowSelected: 'rgba(0,0,0,0.05)',
  waitingTint: 'rgba(239,68,68,0.08)',
};

// Accepts RN's ColorSchemeName ('light' | 'dark' | 'unspecified' | null);
// anything that isn't explicitly 'light' resolves to the dark palette.
export const paletteFor = (scheme?: string | null): Palette =>
  scheme === 'light' ? light : dark;

export interface Section {
  status: SectionKey;
  agents: Agent[];
}

// Group agents into the four sections in fixed rank order, non-empty only. The
// FINISHED (idle) section is ordered most-recently-finished first (`since` desc —
// stable, since an idle agent's `since` is frozen at its last activity); every
// other section is by `primary` case-insensitively. Mirrors the server's sortPanes
// + AgentStore.sections so all surfaces agree.
export function sections(agents: Agent[]): Section[] {
  const out: Section[] = [];
  // ERRORED first among the non-waiting sections: a session that stopped on a failure
  // needs a person, and it will do nothing further until one arrives. It is NOT folded
  // into "needs you" — that section means an agent is asking a question you can answer,
  // and an error is not a question (commander, 2026-08-17).
  const errored = agents
    .filter(a => a.error && a.status === 'idle' && a.source !== 'native' &&
      a.role !== 'supervisor' && !a.watched)
    .sort((l, r) => (r.since ?? 0) - (l.since ?? 0));

  for (const st of SECTION_ORDER) {
    // Native (non-tmux) sessions and WATCHED plain panes are their own trailing
    // categories, not mixed in; the supervisor (role) renders as the HQ card above the
    // list, never a row. A watched pane has no agent status (its "" defaults to
    // 'running' on decode), so without this it would fall into the RUNNING bucket
    // instead of its own "Watched / 关注" section (mirrors the menu-bar).
    const rows = agents
      // An ERRORED idle session is pulled out below — it is not "finished".
      .filter(a => a.status === st && !(st === 'idle' && a.error) &&
        a.source !== 'native' && a.role !== 'supervisor' && !a.watched)
      .sort((l, r) =>
        st === 'idle'
          ? (r.since ?? 0) - (l.since ?? 0)
          : primary(l).toLowerCase().localeCompare(primary(r).toLowerCase()),
      );
    if (rows.length) out.push({status: st, agents: rows});
    // Right after "needs you", before "working".
    if (st === 'waiting' && errored.length) out.push({status: 'errored', agents: errored});
  }
  // "Watched": user-pinned PLAIN panes (tiered-pane-control) — a distinct section, no
  // agent status. Placed above natives (your own pinned tmux panes are nearer than
  // sensed non-tmux sessions).
  const watched = agents
    .filter(a => a.watched && a.role !== 'supervisor')
    .sort((l, r) => primary(l).toLowerCase().localeCompare(primary(r).toLowerCase()));
  if (watched.length) out.push({status: 'watched', agents: watched});
  // "Elsewhere": sensed agents running outside tmux (sense-only, no jump/send).
  const natives = agents
    .filter(a => a.source === 'native' && !a.watched)
    .sort((l, r) => (r.since ?? 0) - (l.since ?? 0));
  if (natives.length) out.push({status: 'native', agents: natives});
  return out;
}

export interface Counts {
  total: number;
  waiting: number;
  working: number;
  /** Idle sessions whose turn ended on a failure — counted apart from idle. */
  errored: number;
  idle: number;
}

export function counts(agents: Agent[]): Counts {
  // Per-status counts DESCRIBE the sections below, which exclude the supervisor
  // (it renders as the HQ card) — so exclude it here too or the summary reads
  // "10 idle" over an IDLE-9 section. WATCHED plain panes are NOT agents (they render
  // in their own section, and their "" status defaults to 'running'), so exclude them
  // too — otherwise a pinned pane inflates "idle" past the IDLE section's count. The
  // TOTAL counts agents (incl. HQ) but not watched panes.
  const rows = agents.filter(a => a.role !== 'supervisor' && !a.watched);
  const waiting = rows.filter(a => a.status === 'waiting').length;
  const working = rows.filter(a => a.status === 'working').length;
  // Errored sessions are counted SEPARATELY, not as idle — the summary has to agree
  // with the sections under it, and a session that stopped on a failure reading as
  // "idle" is exactly the thing that hid it.
  const errored = rows.filter(a => a.error && a.status === 'idle').length;
  return {
    total: agents.filter(a => !a.watched).length,
    waiting, working, errored,
    idle: rows.length - waiting - working - errored,
  };
}
