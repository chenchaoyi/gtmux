import type { TraceEvent, TraceStep, TraceContext } from './model';
import { contextLabel, build, groupTurns, turnBookmarkKey } from './model';
import type { TranscriptTurn } from '../api/client';
import {
  activityOperation,
  activityLabel,
  activityPreview,
} from '../ui/activity';
import { callTitle } from '../ui/toolPresentation';
import { tracePreview, tracePreviewSource } from './timeline';

export const traceKindLabels = {
  all: ['All records', '全部记录'],
  user: ['Messages', '用户消息'],
  assistant: ['Agent replies', 'Agent 回复'],
  tool: ['Tool calls', '工具调用'],
  context: ['Context', '上下文'],
};
export const traceDetailLabels = {
  overview: ['Overview', '概述'],
  preview: ['Preview', '预览'],
  raw: ['Raw content', '原始内容'],
  origin: ['Source', '来源'],
};
export function recordedStep(event: TraceEvent) {
  const step = event.step as TraceStep;
  return {
    kind: 'tool',
    call: step.call,
    workdir: step.workdir,
    input_ref: step.inputRef,
    details_truncated: step.truncated,
    title: event.title,
    input: step.input,
    detail: step.summary,
    operation: step.operation,
    action: step.action,
    subject_count: step.subjectCount,
    input_kind: step.inputKind,
  };
}
export function traceEventLabel(event: TraceEvent, lang: 'en' | 'zh') {
  if (event.kind === 'context') return contextLabel(event, lang);
  return event.kind === 'tool'
    ? callTitle(recordedStep(event), lang) || activityLabel(activityOperation(recordedStep(event)), lang)
    : event.kind === 'user'
    ? event.from?.label || (lang === 'zh' ? '请求' : 'Request')
    : lang === 'zh'
    ? 'Agent 回复'
    : 'Agent reply';
}
// Counts describe the loaded window, never a source ordinal or a whole session.
// Both renderers use these labels so context cannot be counted as a message.
export function traceCounts(events: TraceEvent[]) {
  return {
    turns: new Set(events.map(event => event.turnKey)).size,
    calls: events.filter(event => event.kind === 'tool').length,
    messages: events.filter(
      event => event.kind === 'user' || event.kind === 'assistant',
    ).length,
    contexts: events.filter(event => event.kind === 'context').length,
    records: events.length,
  };
}
export function traceWindowLabel(events: TraceEvent[], lang: 'en' | 'zh') {
  const counts = traceCounts(events);
  return lang === 'zh'
    ? `${counts.turns} 轮 · ${counts.records} 条记录`
    : `${counts.turns} ${counts.turns === 1 ? 'turn' : 'turns'} · ${
        counts.records
      } ${counts.records === 1 ? 'record' : 'records'}`;
}
export function traceTurnSummary(events: TraceEvent[], lang: 'en' | 'zh') {
  const counts = traceCounts(events);
  const parts =
    lang === 'zh'
      ? [`${counts.calls} 次调用`, `${counts.messages} 条消息`]
      : [
          `${counts.calls} ${counts.calls === 1 ? 'call' : 'calls'}`,
          `${counts.messages} ${
            counts.messages === 1 ? 'message' : 'messages'
          }`,
        ];
  if (counts.contexts) {
    parts.push(
      lang === 'zh'
        ? `${counts.contexts} 条上下文`
        : `${counts.contexts} context ${
            counts.contexts === 1 ? 'record' : 'records'
          }`,
    );
  }
  return parts.join(' · ');
}
export function traceDetailMetadata(
  event: TraceEvent,
  lang: 'en' | 'zh',
  time = '',
) {
  return [
    event.agent,
    lang === 'zh' ? `第 ${event.turn} 轮` : `Turn ${event.turn}`,
    time ? (lang === 'zh' ? '本轮开始于 ' : 'Turn started ') + time : '',
  ]
    .filter(Boolean)
    .join(' · ');
}
export function traceEventPreview(event: TraceEvent, lang: 'en' | 'zh') {
  return event.kind === 'tool'
    ? activityPreview(recordedStep(event), lang)
    : tracePreview(event.body, lang);
}
export function traceEventIcon(event: TraceEvent) {
  return event.kind === 'tool'
    ? activityOperation(recordedStep(event))
    : event.kind;
}
// Same neutral glyphs for non-tool records in both renderers; sender identities
// use each client's existing shared avatar component when one was recorded.
export const traceRecordPaths = {
  context: 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18 M12 11v6 M12 7v.1',
  user: 'M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0 M4 21v-2a8 8 0 0 1 16 0v2',
  assistant: 'm12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5L12 3Z',
};

// Loaded-window navigation reuses trace identity, original ordinals and summaries.
export function conversationOutline(
  turns: TranscriptTurn[],
  lang: 'en' | 'zh' = 'en',
  context?: TraceContext,
) {
  return groupTurns(build(turns, context).events).map(group => ({
    key: group.key,
    bookmarkKey: turnBookmarkKey(turns[group.events[0].sourceIndex], context),
    sourceIndex: group.events[0].sourceIndex,
    turn: group.turn,
    agent: group.agent,
    session: group.session,
    prompt: tracePreview(group.prompt?.body || group.events[0].body, lang),
    promptMarkdown: tracePreviewSource(
      group.prompt?.body || group.events[0].body,
      lang,
    ).slice(0, 8192),
    summary: tracePreview(
      [...group.events].reverse().find(event => event.kind === 'assistant')
        ?.body || '',
      lang,
    ),
    summaryMarkdown: tracePreviewSource(
      [...group.events].reverse().find(event => event.kind === 'assistant')
        ?.body || '',
      lang,
    ).slice(0, 8192),
  }));
}
