// One public-transcript index for native readers and the generated browser asset.
import type { TranscriptSender, TranscriptTurn } from '../api/client';

export type TraceKind = 'user' | 'assistant' | 'tool';
export interface TraceContext {
  home_id?: string;
  session_id?: string;
  pane_id?: string;
  agent?: string;
}
export interface TraceStep {
  summary: string;
  input: string;
  inputKind: string;
  workdir: string;
  operation: string;
  truncated: boolean;
}
export interface TraceEvent {
  key: string;
  turnKey: string;
  turn: number;
  session: string;
  sessionKey: string;
  agent: string;
  kind: TraceKind;
  title: string;
  body: string;
  time: string;
  from: TranscriptSender | null;
  boundary: TranscriptTurn['session_break'] | null;
  step: TraceStep | null;
  search: string;
  sourceIndex: number;
}
export interface TraceSession {
  key: string;
  session: string;
  agent: string;
  count: number;
}
export interface TraceIndex {
  events: TraceEvent[];
  sessions: TraceSession[];
  turns: number;
}
const string = (value: unknown): string =>
  typeof value === 'string' ? value : '';
const list = (value: unknown): unknown[] => (Array.isArray(value) ? value : []);
function object(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object'
    ? (value as Record<string, unknown>)
    : null;
}
/* eslint-disable no-bitwise -- Stable identity uses two explicit 32-bit hashes. */
function digest(value: string): string {
  let a = 2166136261,
    b = 5381;
  for (let i = 0; i < value.length; i++) {
    a = Math.imul(a ^ value.charCodeAt(i), 16777619);
    b = Math.imul(b, 33) ^ value.charCodeAt(i);
  }
  return (a >>> 0).toString(36) + '-' + (b >>> 0).toString(36);
}
/* eslint-enable no-bitwise */
export function agentName(value: unknown): string {
  const name = string(value).trim();
  const brands: Record<string, string> = {
    codex: 'Codex',
    claude: 'Claude Code',
    'claude code': 'Claude Code',
    dsh: 'DeepSeek Harness',
    'deepseek harness': 'DeepSeek Harness',
    kimi: 'Kimi',
    opencode: 'OpenCode',
  };
  return Object.prototype.hasOwnProperty.call(brands, name.toLowerCase())
    ? brands[name.toLowerCase()]
    : name;
}
export function build(
  turns: unknown,
  context: TraceContext | null = {},
): TraceIndex {
  const ctx = context || {};
  const events: TraceEvent[] = [],
    sessions: TraceSession[] = [];
  const groups = new Map<string, TraceSession>(),
    occurrences = new Map<string, number>(),
    turnCounts = new Map<string, number>();
  list(turns).forEach((raw, sourceIndex) => {
    const turn = object(raw);
    if (!turn) return;
    const session =
      string(turn.session_id) ||
      string(ctx.session_id) ||
      string(ctx.pane_id) ||
      'current';
    const scope = string(ctx.home_id) + '\x00' + session,
      agent = agentName(turn.agent || ctx.agent);
    let group = groups.get(scope);
    if (!group) {
      group = { key: digest(scope), session, agent, count: 0 };
      groups.set(scope, group);
      sessions.push(group);
    }
    const sessionGroup = group;
    const ordinal = (turnCounts.get(scope) || 0) + 1;
    turnCounts.set(scope, ordinal);
    const prompt = string(turn.prompt),
      time = string(turn.time),
      identity = scope + '\x00' + time + '\x00' + prompt;
    const occurrence = occurrences.get(identity) || 0;
    occurrences.set(identity, occurrence + 1);
    const key = digest(identity) + '-' + occurrence;
    const sender = object(turn.from);
    const from =
      sender && (sender.kind === 'hq' || sender.kind === 'agent')
        ? ({
            kind: sender.kind,
            label: string(sender.label),
            agent: string(sender.agent),
            pane: string(sender.pane),
          } as TranscriptSender)
        : null;
    function push(
      kind: TraceKind,
      position: string,
      body: string,
      title: string,
      step: TraceStep | null,
    ) {
      const event: TraceEvent = {
        key: key + ':' + position,
        turnKey: key,
        turn: ordinal,
        session,
        sessionKey: sessionGroup.key,
        agent,
        kind,
        title,
        body,
        time,
        from: kind === 'user' ? from : null,
        boundary:
          (object(turn!.session_break) as TranscriptTurn['session_break']) ||
          null,
        step,
        sourceIndex,
        search: [title, body, agent, session, step?.workdir, step?.operation]
          .filter(Boolean)
          .join('\n')
          .toLocaleLowerCase(),
      };
      events.push(event);
      sessionGroup.count++;
    }
    if (prompt) push('user', 'prompt', prompt, string(sender?.label), null);
    let segments = list(turn.segments);
    if (!segments.length && string(turn.response))
      segments = [{ text: turn.response }];
    segments.forEach((rawSegment, index) => {
      const segment = object(rawSegment);
      if (!segment) return;
      if (string(segment.text))
        push('assistant', index + ':text', string(segment.text), agent, null);
      list(segment.steps).forEach((rawStep, offset) => {
        const step = object(rawStep);
        if (!step) return;
        const input = string(step.input),
          detail = string(step.detail);
        push(
          'tool',
          index + ':tool:' + offset,
          input || detail,
          string(step.title),
          {
            summary: detail,
            input,
            inputKind: string(step.input_kind),
            workdir: string(step.workdir),
            operation: string(step.operation),
            truncated: step.details_truncated === true,
          },
        );
      });
    });
  });
  return {
    events,
    sessions,
    turns: Array.from(turnCounts.values()).reduce((a, b) => a + b, 0),
  };
}
export interface TraceTurn {
  key: string;
  sessionKey: string;
  session: string;
  agent: string;
  turn: number;
  time: string;
  prompt: TraceEvent | null;
  events: TraceEvent[];
  calls: number;
}

/** Group only recorded order and identity; no outcome or timing is inferred. */
export function groupTurns(events: TraceEvent[]): TraceTurn[] {
  const groups = new Map<string, TraceTurn>();
  for (const event of events) {
    let group = groups.get(event.turnKey);
    if (!group) {
      group = {
        key: event.turnKey,
        sessionKey: event.sessionKey,
        session: event.session,
        agent: event.agent,
        turn: event.turn,
        time: event.time,
        prompt: null,
        events: [],
        calls: 0,
      };
      groups.set(event.turnKey, group);
    }
    group.events.push(event);
    if (event.kind === 'user') group.prompt = event;
    if (event.kind === 'tool') group.calls++;
  }
  return Array.from(groups.values());
}
export function filter(
  events: TraceEvent[],
  query = '',
  kind: TraceKind | 'all' = 'all',
  session = 'all',
): TraceEvent[] {
  const needle = string(query).trim().toLocaleLowerCase();
  return events.filter(
    event =>
      (!kind || kind === 'all' || event.kind === kind) &&
      (!session || session === 'all' || event.sessionKey === session) &&
      (!needle || event.search.includes(needle)),
  );
}
export function overview(
  events: TraceEvent[],
  maximum = 120,
): { from: number; to: number; kind: TraceKind | 'mixed' }[] {
  const total = events.length,
    size = Math.max(1, Math.min(240, maximum || 120)),
    out: { from: number; to: number; kind: TraceKind | 'mixed' }[] = [];
  for (let i = 0, count = Math.min(total, size); i < count; i++) {
    const from = Math.floor((i * total) / count),
      to = Math.floor(((i + 1) * total) / count);
    let kind: TraceKind | 'mixed' = events[from].kind;
    for (let j = from + 1; j < to; j++)
      if (events[j].kind !== kind) {
        kind = 'mixed';
        break;
      }
    out.push({ from, to, kind });
  }
  return out;
}
