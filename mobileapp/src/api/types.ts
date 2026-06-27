// Mirrors macapp/Sources/GtmuxBar/AgentStore.swift's `Agent` (and the
// `agentJSON` shape in internal/app/agents.go) — the one cross-surface contract.
// Tolerate missing fields: default status "running", source "tmux".

export type StatusName = 'waiting' | 'working' | 'idle' | 'running';

export interface Agent {
  pane_id: string;
  session: string;
  window: string;
  pane: string;
  loc: string;
  agent: string;
  status: StatusName;
  task: string;
  latest: boolean;
  activity: boolean;
  source: string; // "tmux" | "native"
  project?: string;
  terminal?: string;
  tab?: string;
  activity_at?: number;
  since?: number;
  icon?: string;
}

// Decode one agent from raw JSON, applying the same defaults as the Swift decoder.
export function toAgent(raw: any): Agent {
  const s = (k: string) => (typeof raw?.[k] === 'string' ? raw[k] : '');
  const b = (k: string) => raw?.[k] === true;
  const n = (k: string) => (typeof raw?.[k] === 'number' ? raw[k] : undefined);
  const status = (raw?.status as StatusName) || 'running';
  return {
    pane_id: s('pane_id'),
    session: s('session'),
    window: s('window'),
    pane: s('pane'),
    loc: s('loc'),
    agent: s('agent'),
    status,
    task: s('task'),
    latest: b('latest'),
    activity: b('activity'),
    source: s('source') || 'tmux',
    project: s('project') || undefined,
    terminal: s('terminal') || undefined,
    tab: s('tab') || undefined,
    activity_at: n('activity_at'),
    since: n('since'),
    icon: s('icon') || undefined,
  };
}

// A stable identity for list keys (mirrors Agent.id in Swift).
export const agentId = (a: Agent): string =>
  a.pane_id || `${a.source}:${a.terminal}:${a.tab}:${a.project}:${a.agent}`;

const isNative = (a: Agent) => a.source === 'native';

// Row line 1 (bold): the agent's OWN session/task title, NOT a cwd project.
export const primary = (a: Agent): string => {
  if (a.task) return a.task;
  if (isNative(a)) return a.project || a.terminal || '';
  return a.session || a.loc;
};

// Row line 2 (dim): where it lives — "session · %pane", or the native terminal.
export const secondary = (a: Agent): string => {
  if (isNative(a)) return a.terminal || '';
  const base = a.session || a.loc;
  return a.pane_id ? `${base} · ${a.pane_id}` : base;
};

export interface Alert {
  pane: string;
  kind: 'waiting' | 'done';
  agent: string;
  loc: string;
  task: string;
}

// One parsed interactive choice from a waiting pane (GET /api/options) — the
// number you'd press (1/2/3) and the agent's own label for it. Approval card.
export interface ReplyOption {
  n: number;
  label: string;
}

export interface PaneResponse {
  id: string;
  text: string;
  // the pane's text cursor (xterm renderer positions it): column x, Up = rows above
  // the last captured line, visible = false in alt-screen TUIs that hide the cursor.
  cursor?: {x: number; up: number; visible: boolean};
}

// The host terminal's resolved appearance (GET /api/theme) — colors + font, so the
// pane mirror matches the user's real terminal. Palette is the 16 ANSI colors.
export interface TermTheme {
  source: string;
  background: string;
  foreground: string;
  cursor: string;
  palette: string[];
  fontFamily: string;
  fontSize: number;
}
