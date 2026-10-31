// One public-transcript index for native readers and the generated browser asset.
import type { TranscriptSender, TranscriptTurn } from '../api/client';

export type TraceKind = 'user' | 'assistant' | 'tool' | 'context';
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
  inputRef?: string;
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
  context: { source: string; time: string; truncated: boolean } | null;
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
  // Old servers can omit both clocks and record IDs. Only ambiguous prompts
  // need a content fallback; a single streaming turn must retain its identity.
  const clockless = new Map<string, number>();
  list(turns).forEach(raw => {
    const turn = object(raw);
    if (!turn || string(turn.record_id) || string(turn.time)) return;
    const scope =
      string(ctx.home_id) +
      '\x00' +
      (string(turn.session_id) ||
        string(ctx.session_id) ||
        string(ctx.pane_id) ||
        'current');
    const value = scope + '\x00' + string(turn.prompt);
    clockless.set(value, (clockless.get(value) || 0) + 1);
  });
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
      identity =
        scope +
        '\x00' +
        (string(turn.record_id) || time) +
        '\x00' +
        prompt +
        (!string(turn.record_id) &&
        !time &&
        (clockless.get(scope + '\x00' + prompt) || 0) > 1
          ? '\x00' + JSON.stringify([turn.response, turn.segments])
          : '');
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
      contextRecord: TraceEvent['context'] = null,
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
        context: contextRecord,
        sourceIndex,
        search: [
          title,
          body,
          agent,
          session,
          step?.workdir,
          step?.operation,
          contextRecord?.source,
        ]
          .filter(Boolean)
          .join('\n')
          .toLocaleLowerCase(),
      };
      events.push(event);
      sessionGroup.count++;
    }
    let segments = list(turn.segments);
    if (!segments.length && string(turn.response))
      segments = [{ text: turn.response }];
    function pushContext(rawSegment: unknown, index: number, before: boolean) {
      const contextRecord = object(object(rawSegment)?.context);
      if (
        contextRecord &&
        (contextRecord.before_prompt === true) === before &&
        string(contextRecord.text)
      )
        push(
          'context',
          index + ':context',
          string(contextRecord.text),
          string(contextRecord.title),
          null,
          {
            source: string(contextRecord.source),
            time: string(contextRecord.time),
            truncated: contextRecord.truncated === true,
          },
        );
    }
    segments.forEach((segment, index) => pushContext(segment, index, true));
    if (prompt) push('user', 'prompt', prompt, string(sender?.label), null);
    segments.forEach((rawSegment, index) => {
      const segment = object(rawSegment);
      if (!segment) return;
      pushContext(segment, index, false);
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
            inputRef: string(step.input_ref),
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

/** Recorded provenance only. This is not the provider's original request JSON. */
export function sourceInfo(event: TraceEvent): Record<string, unknown> {
  return {
    format: 'gtmux transcript',
    kind: event.kind,
    agent: event.agent,
    session_id: event.session,
    turn: event.turn,
    record_source:
      event.context?.source ||
      (event.kind === 'context'
        ? 'segments.context'
        : event.kind === 'tool'
        ? 'segments.steps'
        : event.kind === 'user'
        ? 'prompt'
        : 'segments.text'),
    ...(event.time ? { turn_started_at: event.time } : {}),
    ...(event.context?.time ? { recorded_at: event.context.time } : {}),
    ...(event.from ? { sender: event.from } : {}),
    ...(event.step?.workdir ? { workdir: event.step.workdir } : {}),
    ...(event.step
      ? {
          input_kind: event.step.inputKind,
          operation: event.step.operation,
          truncated: event.step.truncated,
        }
      : {}),
    ...(event.context ? { truncated: event.context.truncated } : {}),
  };
}

export function contextLabel(event: TraceEvent, lang: 'en' | 'zh'): string {
  const names: Record<string, [string, string]> = {
    'Workspace instructions': ['Workspace instructions', '工作区指令'],
    Environment: ['Environment', '运行环境'],
    'Environment reminder': ['Environment reminder', '环境提醒'],
    'Background task notification': [
      'Background task notification',
      '后台任务通知',
    ],
    'Conversation summary': ['Conversation summary', '会话摘要'],
    'Harness input': ['Context', '上下文'],
  };
  return (
    names[event.title]?.[lang === 'zh' ? 1 : 0] ||
    event.title ||
    (lang === 'zh' ? '上下文' : 'Context')
  );
}

export function overviewFacts(
  event: TraceEvent,
  lang: 'en' | 'zh',
): { label: string; value: string }[] {
  const zh = lang === 'zh',
    kinds = {
      user: ['Message', '用户消息'],
      assistant: ['Agent reply', 'Agent 回复'],
      tool: ['Tool call', '工具调用'],
      context: ['Context', '上下文'],
    };
  const facts = [
    { label: zh ? '类型' : 'Type', value: kinds[event.kind][zh ? 1 : 0] },
    { label: zh ? '轮次' : 'Turn', value: String(event.turn) },
    { label: 'Agent', value: event.agent },
    { label: zh ? '会话' : 'Session', value: event.session },
  ];
  if (event.context?.time)
    facts.push({
      label: zh ? '记录时间' : 'Recorded at',
      value: event.context.time,
    });
  if (event.step?.workdir)
    facts.push({
      label: zh ? '工作目录' : 'Working directory',
      value: event.step.workdir,
    });
  return facts.filter(fact => fact.value);
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

export type TimelineRow =
  | { key: string; type: 'turn'; group: TraceTurn; open: boolean }
  | {
      key: string;
      type: 'event';
      event: TraceEvent;
      order: number;
      last: boolean;
    }
  | {
      key: string;
      type: 'calls';
      events: TraceEvent[];
      open: boolean;
      last: boolean;
    };

export function timelineRows(
  groups: TraceTurn[],
  expanded: Record<string, boolean>,
  calls: Record<string, boolean>,
  searching = false,
  defaults: { turns?: boolean; calls?: boolean } = {},
): TimelineRow[] {
  const rows: TimelineRow[] = [];
  for (const group of groups) {
    const open = searching || (expanded[group.key] ?? defaults.turns ?? false);
    rows.push({ key: 'turn:' + group.key, type: 'turn', group, open });
    if (!open) continue;
    for (let i = 0; i < group.events.length; i++) {
      const event = group.events[i];
      let end = i + 1;
      if (!searching && event.kind === 'tool') {
        while (end < group.events.length && group.events[end].kind === 'tool')
          end++;
      }
      if (
        !searching &&
        event.kind === 'tool' &&
        end - i >= (defaults.calls === undefined ? 3 : 1)
      ) {
        const key = 'calls:' + event.key,
          unfolded = calls[key] ?? defaults.calls ?? false;
        rows.push({
          key,
          type: 'calls',
          events: group.events.slice(i, end),
          open: unfolded,
          last: end === group.events.length && !unfolded,
        });
        if (!unfolded) {
          i = end - 1;
          continue;
        }
        for (let j = i; j < end; j++)
          rows.push({
            key: group.events[j].key,
            type: 'event',
            event: group.events[j],
            order: j + 1,
            last: j === group.events.length - 1,
          });
        i = end - 1;
      } else
        rows.push({
          key: event.key,
          type: 'event',
          event,
          order: i + 1,
          last: i === group.events.length - 1,
        });
    }
  }
  return rows;
}

/** Reading transforms only; raw content and copy retain the recorded text. */
export function previewText(event: TraceEvent): string {
  if (
    event.kind === 'tool' &&
    ['json', 'arguments'].includes(event.step?.inputKind || '')
  ) {
    try {
      const value = JSON.parse(event.body);
      if (value !== null && typeof value === 'object')
        return JSON.stringify(value, null, 2);
    } catch {
      /* Older records may contain partial JSON. */
    }
  }
  if (event.kind === 'context') {
    const envelope =
      /^\s*<(system-reminder|task-notification)\b[^>]*>([\s\S]*)<\/\1>\s*$/.exec(
        event.body,
      );
    if (envelope) return envelope[2].trim();
  }
  return event.body;
}
