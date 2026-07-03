// Design tokens — mirrors macapp/.../Theme.swift. Status colors are the
// AUTHORITATIVE hex (DESIGN §1/§9); keep identical across all three surfaces.

import {Agent, StatusName, primary} from '../api/types';

export const StatusColor: Record<StatusName, string> = {
  waiting: '#EF4444', // red
  working: '#06B6D4', // cyan
  idle: '#22C55E', // green
  running: '#8E8E93', // gray (none / running)
};

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
  status: StatusName;
  agents: Agent[];
}

// Group agents into the four sections in fixed rank order, non-empty only. The
// FINISHED (idle) section is ordered most-recently-finished first (`since` desc —
// stable, since an idle agent's `since` is frozen at its last activity); every
// other section is by `primary` case-insensitively. Mirrors the server's sortPanes
// + AgentStore.sections so all surfaces agree.
export function sections(agents: Agent[], waitingOnly: boolean): Section[] {
  const out: Section[] = [];
  for (const st of SECTION_ORDER) {
    if (waitingOnly && st !== 'waiting') continue;
    const rows = agents
      .filter(a => a.status === st)
      .sort((l, r) =>
        st === 'idle'
          ? (r.since ?? 0) - (l.since ?? 0)
          : primary(l).toLowerCase().localeCompare(primary(r).toLowerCase()),
      );
    if (rows.length) out.push({status: st, agents: rows});
  }
  return out;
}

export interface Counts {
  total: number;
  waiting: number;
  working: number;
  idle: number;
}

export function counts(agents: Agent[]): Counts {
  const waiting = agents.filter(a => a.status === 'waiting').length;
  const working = agents.filter(a => a.status === 'working').length;
  return {total: agents.length, waiting, working, idle: agents.length - waiting - working};
}
