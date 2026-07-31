// Mirrors macapp/Sources/GtmuxBar/AgentStore.swift's `Agent` (and the
// `agentJSON` shape in internal/radar/agents.go) — the one cross-surface contract.
// Tolerate missing fields: default status "running", source "tmux".

export type StatusName = 'waiting' | 'working' | 'idle' | 'running';
// Section grouping key: the four statuses plus the non-tmux ("Elsewhere") category.
export type SectionKey = StatusName | 'native' | 'watched';

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
  // "supervisor" marks the hq (中控) session — its own HQ card, never a section row.
  role?: string;
  project?: string;
  branch?: string; // git branch of the pane's cwd (radar++)
  terminal?: string;
  tab?: string;
  activity_at?: number;
  since?: number;
  icon?: string;
  // errored-idle modifier: this idle session ended on an API/tool error. Surfaces
  // mark it with an amber ⚠ (NOT red — red is waiting). false/absent = finished ok.
  error?: boolean;
  error_text?: string;
  // background-running modifier: this idle session's turn ended with background
  // work still in flight. Marked with an amber ⧗ (NOT red). false/absent = done.
  bg?: boolean;
  bg_count?: number;
  bg_text?: string;
  // watched (tiered-pane-control): a user-promoted PLAIN pane, not an agent — no
  // agent status. Absent/false for agent rows.
  watched?: boolean;
}

// PaneRow mirrors `gtmux panes --json` (GET /api/panes) — EVERY tmux pane, tiered
// agent/plain (tiered-pane-control). The superset of Agent used by the pane browser
// and the Detail neighbor strip; focus/send/attach work on any of them.
export interface PaneRow {
  pane_id: string;
  loc: string;
  session: string;
  window: string;
  pane: string;
  cwd?: string;
  command: string;
  title?: string;
  active?: boolean;
  in_mode?: boolean;
  tier: 'agent' | 'plain';
  agent?: string;
  icon?: string; // official-icon hint for an agent pane → the browser avatar (empty for plain)
}

// paneRowToAgent adapts a PaneRow into an Agent so a plain (non-agent) pane can open
// in the same DetailView (its live screen + input). A plain pane has no agent status,
// so status stays 'running' (the neutral bucket); the label is its title or command.
export function paneRowToAgent(r: PaneRow): Agent {
  return {
    pane_id: r.pane_id,
    session: r.session,
    window: r.window,
    pane: r.pane,
    loc: r.loc,
    agent: r.tier === 'agent' ? r.agent || '' : '',
    status: 'running',
    task: r.title || r.command,
    latest: false,
    activity: false,
    source: 'tmux',
    project: r.cwd,
    // an agent pane shows its OFFICIAL icon (via /api/icon) like the radar; a plain
    // pane has none → AgentAvatar falls back to the $_ monogram.
    icon: r.tier === 'agent' ? r.icon : undefined,
  };
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
    role: s('role') || undefined,
    project: s('project') || undefined,
    branch: s('branch') || undefined,
    terminal: s('terminal') || undefined,
    tab: s('tab') || undefined,
    activity_at: n('activity_at'),
    since: n('since'),
    icon: s('icon') || undefined,
    error: b('error') || undefined,
    error_text: s('error_text') || undefined,
    bg: b('bg') || undefined,
    bg_count: n('bg_count'),
    bg_text: s('bg_text') || undefined,
    // watched (tiered-pane-control): a user-pinned PLAIN pane — decoded so it lands in
    // the "Watched / 关注" section, NOT the RUNNING bucket (its "" status defaults to
    // 'running'). Without this the section fix never fires.
    watched: b('watched') || undefined,
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
  if (isNative(a)) return a.terminal || a.agent; // no terminal locator → the agent name
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
  // the pane's text cursor (the terminal renderer positions it): column x, Up = rows above
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

// Server mode — the Mac kept running with the lid closed (openspec change server-mode).
//
// The phone is deliberately asymmetric here: it can SEE this state and can turn it
// OFF, but can never turn it ON. Enabling needs an administrator authorization typed
// at the Mac, and an unattended machine has nobody to answer it — a wrong remote
// enable would burn a laptop's battery in a bag for days.
//
// Every field Go marks `omitempty` is absent when zero, so it is optional here.
export interface ServerMode {
  state: 'on' | 'off' | 'lapsed';
  tier?: string;
  since?: number;
  power: 'ac' | 'battery';
  battery_pct?: number;
  guard: {installed: boolean; healthy: boolean};
  system_disablesleep: boolean;
  owned_by_gtmux: boolean;
  last_exit?: {at: number; reason: string};
  platform: {ok: boolean; verified: boolean; reason?: string; os_version?: string};
}

// serverModeNeedsAttention: red is reserved for "a human should look at this" —
// the same discipline the HQ disc uses, where a soft amber must not read as red.
export function serverModeNeedsAttention(m: ServerMode): boolean {
  if (m.state === 'lapsed') return true;
  if (m.system_disablesleep && !m.guard.healthy) return true;
  if (m.system_disablesleep && m.power === 'battery' && (m.battery_pct ?? 100) <= 30) return true;
  return false;
}
