import {TranscriptStep} from '../api/client';

export type ActivityOperation = 'read' | 'list' | 'search' | 'command' | 'edit' | 'web' | 'code' | 'wait' | 'agent' | 'tool';
const names: Record<string, ActivityOperation> = {
  read: 'read', readfile: 'read', read_file: 'read', read_text_file: 'read',
  ls: 'list', list: 'list', list_files: 'list', list_directory: 'list', glob: 'list',
  grep: 'search', rg: 'search', search: 'search', search_query: 'search', find: 'search', find_files: 'search',
  bash: 'command', shell: 'command', exec: 'command', exec_command: 'command', local_shell: 'command', run_command: 'command',
  edit: 'edit', write: 'edit', apply_patch: 'edit', patch: 'edit', write_file: 'edit', edit_file: 'edit',
  web_search: 'web', webfetch: 'web', web_fetch: 'web', browse: 'web', open_url: 'web', fetch: 'web',
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
  const name = (step.title || '').split(/__|\./).pop()?.toLowerCase() || '';
  return names[name] || 'tool';
}
export function activityLabel(operation: ActivityOperation, lang: 'en' | 'zh'): string {
  return labels[operation][lang === 'zh' ? 1 : 0];
}
export function activitySummary(steps: TranscriptStep[], lang: 'en' | 'zh'): string {
  const operations = [...new Set(steps.map(activityOperation))];
  if (operations.length === 1) { return activityLabel(operations[0], lang); }
  return lang === 'zh' ? '执行过程' : 'Activity';
}
function compactPreview(value: string): string {
  const text = value.replace(/\s+/g, ' ').trim();
  // Bound the spoken label too; the inspector retains the complete recorded input.
  return text.length <= 160 ? text : text.slice(0, 159).replace(/[\uD800-\uDBFF]$/, '') + '…';
}

export function activityPreview(step: TranscriptStep, lang: 'en' | 'zh' = 'en'): string {
  const operation = activityOperation(step);
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
