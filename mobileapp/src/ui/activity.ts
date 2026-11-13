import {TranscriptStep} from '../api/client';

export type ActivityOperation = 'read' | 'list' | 'search' | 'command' | 'edit' | 'web' | 'code' | 'wait' | 'agent' | 'tool';
const names: Record<string, ActivityOperation> = {
  read: 'read', readfile: 'read', read_file: 'read', read_text_file: 'read',
  ls: 'list', list: 'list', list_files: 'list', list_directory: 'list', glob: 'list',
  grep: 'search', rg: 'search', search: 'search', search_query: 'search', find: 'search', find_files: 'search',
  bash: 'command', shell: 'command', exec: 'command', exec_command: 'command', local_shell: 'command', run_command: 'command',
  edit: 'edit', write: 'edit', apply_patch: 'edit', patch: 'edit', write_file: 'edit', edit_file: 'edit',
  web_search: 'web', websearch: 'web', webfetch: 'web', web_fetch: 'web', browse: 'web', open_url: 'web', fetch: 'web',
  js: 'code', python: 'code', wait: 'wait', wait_for: 'wait', waitfor: 'wait', sleep: 'wait', wait_agent: 'wait', taskoutput: 'wait',
  agent: 'agent', task: 'agent', spawn_agent: 'agent', delegate: 'agent', followup_task: 'agent', resume_agent: 'agent',
};
const labels: Record<ActivityOperation, [string, string]> = {
  read: ['Read file', '读取文件'], list: ['List files', '浏览目录'], search: ['Search', '搜索'],
  command: ['Run command', '运行命令'], edit: ['Edit file', '编辑文件'], web: ['Browse web', '访问网页'],
  code: ['Run code', '运行代码'], wait: ['Wait', '等待'], agent: ['Delegate', '委派任务'], tool: ['Tool call', '工具调用'],
};
export function activityOperation(step: TranscriptStep): ActivityOperation {
  if (step.operation && Object.prototype.hasOwnProperty.call(labels, step.operation)) { return step.operation as ActivityOperation; }
  if (step.title === 'web.run' || step.title === 'web__run') return 'web';
  const name = (step.title || '').split(/__|\./).pop()?.toLowerCase() || '';
  return names[name] || 'tool';
}
export function activityLabel(operation: ActivityOperation, lang: 'en' | 'zh'): string {
  return labels[operation][lang === 'zh' ? 1 : 0];
}
// Action/count refine recorded facts; they never override a contradictory base
// operation or turn a single multi-target invocation into several tool calls.
export function activityActionLabel(step: TranscriptStep, lang: 'en' | 'zh'): string | undefined {
  const operation = activityOperation(step), zh = lang === 'zh';
  const count = typeof step.subject_count === 'number' && Number.isSafeInteger(step.subject_count) && step.subject_count >= 1 && step.subject_count <= 10000 ? step.subject_count : 0;
  if (step.action === 'skill' && (operation === 'read' || operation === 'tool') && count <= 1) return zh ? '读取技能' : 'Read skill';
  if (operation === 'read' && count > 1) return zh ? `读取 ${count} 个文件` : `Read ${count} files`;
  if (operation !== 'web') return undefined;
  const method = (step.title || '').split(/__|\./).pop()?.toLowerCase();
  const action = step.action || (['websearch', 'web_search'].includes(method || '') ? 'web_search' : ['webfetch', 'web_fetch', 'open_url', 'fetchurl'].includes(method || '') ? 'web_open' : '');
  if (action === 'web_search') return zh ? '搜索网页' : 'Search web';
  if (action === 'web_open') return zh ? '打开网页' : 'Open webpage';
  if (action === 'web_find') return zh ? '页内查找' : 'Find in page';
  return undefined;
}
export function activitySummary(steps: TranscriptStep[], lang: 'en' | 'zh'): string {
  const titles = [...new Set(steps.map(step => activityActionLabel(step, lang) || activityLabel(activityOperation(step), lang)))];
  if (titles.length === 1) { return titles[0]; }
  return lang === 'zh' ? '执行过程' : 'Activity';
}
function compactPreview(value: string): string {
  const text = value.replace(/\s+/g, ' ').trim();
  // Bound the spoken label too; the inspector retains the complete recorded input.
  return text.length <= 160 ? text : text.slice(0, 159).replace(/[\uD800-\uDBFF]$/, '') + '…';
}

// Decode complete recorded argv for display only. No command is evaluated and
// no read/search meaning is inferred from script text. Raw remains unchanged.
function commandPreview(step: TranscriptStep): string | undefined {
  if (step.input_kind !== 'arguments' || !step.input || step.input.length > 8192 || step.details_truncated) return undefined;
  let value: unknown;
  try { value = JSON.parse(step.input); } catch { return undefined; }
  if (!Array.isArray(value) || value.length === 0 || value.length > 64 || !value.every(v => typeof v === 'string')) return undefined;
  const args = value as string[];
  const shell = args[0].split('/').at(-1);
  if (args.length === 3 && ['sh', 'bash', 'zsh', 'dash', 'ksh'].includes(shell || '') && ['-c', '-lc'].includes(args[1])) return compactPreview(args[2]);
  // A display argv is not a shell command. Quote individual arguments so spaces
  // and metacharacters are not presented as a different invocation.
  return compactPreview(args.map(v => /^[a-zA-Z0-9_./:@%+=,-]+$/.test(v) ? v : JSON.stringify(v)).join(' '));
}

export function activityPreview(step: TranscriptStep, lang: 'en' | 'zh' = 'en'): string {
  const operation = activityOperation(step);
  if (operation === 'command' || ['read', 'search', 'list'].includes(operation)) {
    // A parsed file/query summary is more useful than the transport argv.
    const detail = step.detail?.trim();
    const transportSummary = detail && detail.length >= 16 && step.input?.startsWith(detail.replace(/…$/, ''));
    if (!detail || transportSummary || detail === step.input) {
      const command = commandPreview(step);
      if (command) return command;
    }
  }
  const structured = step.input_kind === 'parameters' ||
    step.input_kind === 'arguments' && (operation === 'agent' || operation === 'wait');
  if (structured && step.input) {
    if (step.detail && !step.detail.trimStart().startsWith('{') && step.detail !== step.title) return compactPreview(step.detail);
    let args: Record<string, unknown> = {};
    // Current cores bound each input to 8 KiB. Older/foreign inputs need no large parse.
    if (step.input.length <= 8192) {
      try {
        const parsed: unknown = JSON.parse(step.input);
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) args = parsed as Record<string, unknown>;
      } catch { /* A partial record stays available in the inspector. */ }
    }
    if (operation === 'agent') {
      for (const key of ['description', 'title', 'message', 'prompt', 'task_name']) {
        const value = args[key];
        if (typeof value === 'string' && value.trim()) return compactPreview(value);
      }
    } else if (operation === 'wait') {
      for (const [key, en, zh] of [
        ['duration_ms', 'Requested', '设定'], ['yield_time_ms', 'Up to', '最多'], ['timeout_ms', 'Timeout setting', '超时设置'],
      ]) {
        const value = args[key];
        if (typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= Number.MAX_SAFE_INTEGER) {
          return lang === 'zh' ? `${zh} ${value / 1000} 秒` : `${en} ${value / 1000}s`;
        }
      }
    }
    return lang === 'zh' ? '已记录参数' : 'Recorded parameters';
  }
  return compactPreview(step.detail || step.input || step.title || 'tool');
}
