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
export function activityPreview(step: TranscriptStep): string {
  return (step.detail || step.input || step.title || 'tool').replace(/\s+/g, ' ').trim();
}
