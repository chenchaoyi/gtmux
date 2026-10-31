import { parseBlocks, type Inline } from '../ui/markdown';
import { agentName } from '../trace/model';
import type { Agent } from '../api/types';
import type {
  HQAssociation,
  MatterState,
  HQWorkspace,
} from '../api/hqWorkspace';
import type { HeaderModel } from './hqHeaderModel';

export type HQMatterCounts = { needsInput: number; following: number };
export function workspaceCounts(
  data: Pick<HQWorkspace, 'matters'>,
): HQMatterCounts {
  const matters = data.matters.filter(m => !m.merged_into);
  return {
    needsInput: matters.filter(m => m.state === 'needs-input').length,
    following: matters.filter(m => m.state === 'open').length,
  };
}
export function workspaceSummary(
  counts: HQMatterCounts,
  zh: boolean,
  separator = ' · ',
): string {
  return (
    zh
      ? [`${counts.needsInput} 项待处理`, `${counts.following} 项跟进中`]
      : [
          `${counts.needsInput} need${
            counts.needsInput === 1 ? 's' : ''
          } input`,
          `${counts.following} ongoing`,
        ]
  ).join(separator);
}
export function consoleHeaderModel(
  base: HeaderModel,
  workspaceMode: boolean,
  counts: HQMatterCounts | undefined,
  zh: boolean,
): HeaderModel {
  if (!workspaceMode) return base;
  return {
    ...base,
    rows: base.rows
      .filter(row => row.key !== 'context')
      .map(row =>
        row.key === 'owed' ? { ...row, label: zh ? '知识计划' : 'Plans' } : row,
      ),
    verdict: counts
      ? workspaceSummary(counts, zh)
      : zh
      ? '对话详情'
      : 'Conversation details',
    urgent: (counts?.needsInput ?? 0) > 0,
  };
}

export function matterState(state: MatterState, zh: boolean): string {
  const names = {
    open: ['Following', '正在跟进'],
    'needs-input': ['Needs input', '待你处理'],
    completed: ['Completed', '已完成'],
    closed: ['Closed', '已关闭'],
  };
  return names[state]?.[zh ? 1 : 0] ?? state;
}
// A recycled pane is never sufficient to open a participant's current session.
export function liveParticipant(
  association: HQAssociation,
  agents: Agent[],
): Agent | undefined {
  if (association.until || association.removed_at) return undefined;
  const c = association.conversation;
  return agents.find(
    a =>
      agentName(a.agent).toLowerCase() === agentName(c.agent).toLowerCase() &&
      a.session_id === c.session_id &&
      (a.home_id ?? '') === (c.home_id ?? ''),
  );
}

export function recentChange(
  kind: string,
  state: MatterState | undefined,
  zh: boolean,
): string {
  const labels: Record<string, string[]> = {
    'hq-result': ['Result', '交付'],
    'hq-decision': ['Decision', '待决问题'],
    'hq-error': ['Issue reported', '问题反馈'],
    'delivery-uncertain': ['Delivery unconfirmed', '送达未确认'],
    register: ['Added to HQ', '已纳入跟进'],
    merge: ['Matters merged', '事项已合并'],
    split: ['Matter split', '事项已拆分'],
  };
  return kind === 'update' && state
    ? matterState(state, zh)
    : labels[kind]?.[zh ? 1 : 0] ?? (zh ? '有新进展' : 'Updated');
}

export function matterPreview(source: string): string {
  const spans = (items: Inline[]) =>
    items.map(i => (i.t === 'br' ? ' ' : i.s)).join('');
  return parseBlocks(source)
    .map(block => {
      switch (block.t) {
        case 'h':
        case 'p':
        case 'quote':
          return spans(block.spans);
        case 'code':
          return block.text;
        case 'ul':
        case 'ol':
          return block.items.map(spans).join(' ');
        case 'table':
          return [...block.header, ...block.rows.flat()].map(spans).join(' ');
        default:
          return '';
      }
    })
    .join(' ')
    .replace(/\s+/g, ' ')
    .trim();
}
