import {KnowledgeEntry} from '../api/client';

// Display vocabulary only. Wire values and authorization stay in the existing API.
const topics: Record<string, [string, string]> = {
  accounts: ['Accounts and services', '账号与服务'], workflows: ['Workflows', '操作流程'],
  'best-practices': ['Working principles', '做事原则'], pitfalls: ['Common pitfalls', '避坑提醒'],
  corrections: ['Corrections', '纠正记录'], environment: ['Computer environment', '本机环境'],
};
const kinds: Record<string, [string, string]> = {
  facts: ['Facts', '事实信息'], howto: ['How-to', '操作方法'], pitfalls: ['Pitfalls', '避坑提醒'],
  judgment: ['Judgment criteria', '判断依据'], decisions: ['Decision record', '决策记录'],
};
const origins: Record<string, [string, string]> = {
  correction: ['User correction', '用户纠正'], recurrence: ['Repeated observation', '重复出现'],
  mined: ['Conversation review', '会话整理'], capture: ['Agent note', 'Agent 记录'], self: ['HQ observation', 'HQ 观察'],
};
const uses: Record<string, [string, string]> = {
  hq: ['HQ standing rules', 'HQ 固定规则'], machine: ['Agents on this computer', '本机 Agent'],
  repo: ['Agents in one project', '指定项目的 Agent'], everyone: ['gtmux product feedback', 'gtmux 产品改进建议'],
};
const word = (table: Record<string, [string, string]>, value: string, zh: boolean) => table[value]?.[zh ? 1 : 0] ?? value;
export const knowledgeTopic = (value: string, zh: boolean) => word(topics, value, zh);
export const knowledgeKind = (value: string, zh: boolean) => word(kinds, value, zh);
export const knowledgeProvenance = (value: string, zh: boolean) => word(origins, value, zh);
export const knowledgeUse = (value: string, zh: boolean) => word(uses, value, zh);

export function knowledgeUsage(e: KnowledgeEntry, zh: boolean): {title: string; state: string; explanation: string; line: string} {
  const pending = !!e.promoted_at && !e.landed_at;
  let title: string, state = '', explanation: string;
  if (!e.audience || !uses[e.audience]) {
    const unknown = pending || !!e.landed_at || !!e.audience;
    title = unknown ? (zh ? '用途待确认' : 'Use not recorded') : (zh ? 'HQ 按需查阅' : 'HQ reference');
    explanation = unknown
      ? (zh ? '这条旧记录没有注明可识别的用途。请先核对应用记录，再选择新的使用方案。' : 'This older record has no recognized use. Review its destination before choosing a new plan.')
      : (zh ? '供 HQ 查找、派活时参考，尚未设为固定指令。' : 'HQ can look this up and refer to it when assigning work. It has not been set as a standing instruction.');
  } else {
    title = knowledgeUse(e.audience, zh);
    state = pending ? (e.audience === 'everyone' ? (zh ? '待提交反馈' : 'Feedback pending') : (zh ? '待完成写入' : 'Write pending'))
      : e.landed_at ? (zh ? '已记录应用' : 'Application recorded') : (zh ? '已指定用途' : 'Use selected');
    explanation = pending
      ? e.audience === 'everyone'
        ? (zh ? '准备给 gtmux 项目的改进反馈，由你审阅后提交。不会公开整个知识库，也不会立即改变其他用户的规则。' : 'Prepare feedback for the gtmux project for you to review and submit. This does not publish your knowledge base or change every user’s rules.')
        : e.audience === 'hq'
          ? (zh ? '加入 HQ 每次加载的固定指令。适合派活、审阅、何时请你确认等协作约定。' : 'Add to HQ’s standing instructions. Use for delegation, review and when HQ should ask you to decide.')
          : e.audience === 'repo'
            ? (zh ? '写入指定项目的指令文件。适合该项目的构建、测试和代码约定；不会自动提交文件。' : 'Write into the selected project’s instructions. Files are not committed automatically.')
            : (zh ? '通过本机 Agent 的指令文件提供。适合共同的环境事实与操作注意事项；已有会话可能需要重新加载。' : 'Make available through local agents’ instruction files. Running sessions may need to reload.')
      : (zh ? '应用结果见记录详情；这不代表运行中的 Agent 已加载，也不代表产品建议已发布。' : 'The ledger records the result below. It does not confirm that running agents loaded it or that product feedback has shipped.');
  }
  return {title, state, explanation, line: state ? `${title} · ${state}` : title};
}
