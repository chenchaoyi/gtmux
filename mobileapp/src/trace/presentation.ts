import type { TraceEvent, TraceStep } from './model';
import { contextLabel, build, groupTurns } from './model';
import type { TranscriptTurn } from '../api/client';
import {
  activityOperation,
  activityLabel,
  activityPreview,
} from '../ui/activity';
import { tracePreview } from './timeline';

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
    title: event.title,
    input: step.input,
    detail: step.summary,
    operation: step.operation,
    input_kind: step.inputKind,
  };
}
export function traceEventLabel(event: TraceEvent, lang: 'en' | 'zh') {
  if (event.kind === 'context') return contextLabel(event, lang);
  return event.kind === 'tool'
    ? activityLabel(activityOperation(recordedStep(event)), lang)
    : event.kind === 'user'
    ? event.from?.label || (lang === 'zh' ? '请求' : 'Request')
    : lang === 'zh'
    ? '进展记录'
    : 'Progress';
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
) {
  return groupTurns(build(turns).events).map(group => ({
    key: group.key,
    sourceIndex: group.events[0].sourceIndex,
    turn: group.turn,
    agent: group.agent,
    session: group.session,
    prompt: tracePreview(group.prompt?.body || group.events[0].body, lang),
    summary: tracePreview(
      [...group.events].reverse().find(event => event.kind === 'assistant')
        ?.body || '',
      lang,
    ),
  }));
}
