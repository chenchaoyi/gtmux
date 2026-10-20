import type {Agent} from '../api/types';
import type {TranscriptTurn, TranscriptSender} from '../api/client';

import type {ParticipantProfile, ProfileFact} from '../api/types';
export type {ParticipantProfile, ProfileFact} from '../api/types';
export interface ProfileTarget {kind: 'human' | 'agent' | 'sender'; agent: Agent; turn?: TranscriptTurn; sender?: TranscriptSender}

const labels: Record<string, [string, string]> = {
  model: ['Model', '模型'], version: ['Session version', '会话版本'], cursor_version: ['Cursor version', 'Cursor 版本'],
  provider: ['Provider', '供应商'], effort: ['Reasoning effort', '推理强度'], mode: ['Task mode', '任务模式'],
  permission: ['Permissions', '权限策略'], sandbox: ['Sandbox', '沙箱'], role: ['Agent role', 'Agent 角色'],
  variant: ['Model variant', '模型变体'], model_alias: ['Model alias', '模型别名'], client: ['Terminal client', '终端客户端'],
  client_version: ['Client version', '客户端版本'], subscription: ['Subscription', '账户套餐'],
  thinking: ['Thinking', '思考'], thinking_level: ['Thinking level', '思考等级'], thinking_budget: ['Thinking budget', '思考预算'], context: ['Context option', '上下文选项'],
};
export function factLabel(key: string, zh: boolean): string {return labels[key]?.[zh ? 1 : 0] ?? key;}
export function displayFacts(profile?: ParticipantProfile): ProfileFact[] {
  return (profile?.facts ?? []).filter(f => labels[f.key] && f.value).sort((a, b) => Object.keys(labels).indexOf(a.key) - Object.keys(labels).indexOf(b.key));
}
// Source session and time, rather than an array index, survive prepended history.
export function turnIdentity(turn: TranscriptTurn, index: number): string {
  return `${turn.session_id ?? turn.agent ?? ''}:${turn.time ?? `index-${index}`}:${turn.prompt.slice(0, 64)}`;
}
