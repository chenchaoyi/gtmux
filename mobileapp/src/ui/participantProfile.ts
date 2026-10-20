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
// These are display aliases, not provider identity normalization. Custom IDs stay literal.
const providerBrands = new Map([['openai', 'OpenAI'], ['anthropic', 'Anthropic'], ['deepseek', 'DeepSeek'], ['google', 'Google']]);
export function factValue(fact: ProfileFact): string {
  return fact.key === 'provider' ? providerBrands.get(fact.value.toLowerCase()) ?? fact.value : fact.value;
}
export function factSourceLabel(source: string, zh: boolean): string {
  const sources: Record<string, [string, string]> = {
    session_log: ['Session log', '本会话日志'], verified_binding: ['Verified client binding', '已核验的客户端绑定'],
    session_rate_limits: ['Rate limits recorded in this session', '本会话当时的额度记录'],
    runtime_package: ['Runtime package', '运行实例加载包'], native_hook: ['Native hook', '原生 hook'],
  };
  return sources[source]?.[zh ? 1 : 0] ?? (zh ? '已记录来源' : 'Recorded source');
}
export function displayFacts(profile?: ParticipantProfile): ProfileFact[] {
  return (profile?.facts ?? []).filter(f => labels[f.key] && f.value).sort((a, b) => Object.keys(labels).indexOf(a.key) - Object.keys(labels).indexOf(b.key));
}
// Source session and time, rather than an array index, survive prepended history.
export function turnIdentity(turn: TranscriptTurn, index: number): string {
  return `${turn.session_id ?? turn.agent ?? ''}:${turn.time ?? `index-${index}`}:${turn.prompt.slice(0, 64)}`;
}
