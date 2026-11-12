// Generated from shared Activity; SHA256 3a7d33a6204f1ffb2a2937d23a296da447ce5e65a5659bea2cee0867744c61f3
(function(root){var exports={};
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.activityOperation = activityOperation;
exports.activityLabel = activityLabel;
exports.activitySummary = activitySummary;
exports.activityPreview = activityPreview;
const names = {
    read: 'read', readfile: 'read', read_file: 'read', read_text_file: 'read',
    ls: 'list', list: 'list', list_files: 'list', list_directory: 'list', glob: 'list',
    grep: 'search', rg: 'search', search: 'search', search_query: 'search', find: 'search', find_files: 'search',
    bash: 'command', shell: 'command', exec: 'command', exec_command: 'command', local_shell: 'command', run_command: 'command',
    edit: 'edit', write: 'edit', apply_patch: 'edit', patch: 'edit', write_file: 'edit', edit_file: 'edit',
    web_search: 'web', webfetch: 'web', web_fetch: 'web', browse: 'web', open_url: 'web', fetch: 'web',
    js: 'code', python: 'code', wait: 'wait', wait_for: 'wait', waitfor: 'wait', sleep: 'wait', wait_agent: 'wait', taskoutput: 'wait',
    agent: 'agent', task: 'agent', spawn_agent: 'agent', delegate: 'agent', followup_task: 'agent', resume_agent: 'agent',
};
const labels = {
    read: ['Read file', '读取文件'], list: ['List files', '浏览目录'], search: ['Search', '搜索'],
    command: ['Run command', '运行命令'], edit: ['Edit file', '编辑文件'], web: ['Browse web', '访问网页'],
    code: ['Run code', '运行代码'], wait: ['Wait', '等待'], agent: ['Delegate', '委派任务'], tool: ['Tool call', '工具调用'],
};
function activityOperation(step) {
    if (step.operation && Object.prototype.hasOwnProperty.call(labels, step.operation)) {
        return step.operation;
    }
    const name = (step.title || '').split(/__|\./).pop()?.toLowerCase() || '';
    return names[name] || 'tool';
}
function activityLabel(operation, lang) {
    return labels[operation][lang === 'zh' ? 1 : 0];
}
function activitySummary(steps, lang) {
    const operations = [...new Set(steps.map(activityOperation))];
    if (operations.length === 1) {
        return activityLabel(operations[0], lang);
    }
    return lang === 'zh' ? '执行过程' : 'Activity';
}
function compactPreview(value) {
    const text = value.replace(/\s+/g, ' ').trim();
    // Bound the spoken label too; the inspector retains the complete recorded input.
    return text.length <= 160 ? text : text.slice(0, 159).replace(/[\uD800-\uDBFF]$/, '') + '…';
}
// Decode complete recorded argv for display only. No command is evaluated and
// no read/search meaning is inferred from script text. Raw remains unchanged.
function commandPreview(step) {
    if (step.input_kind !== 'arguments' || !step.input || step.input.length > 8192 || step.details_truncated)
        return undefined;
    let value;
    try {
        value = JSON.parse(step.input);
    }
    catch {
        return undefined;
    }
    if (!Array.isArray(value) || value.length === 0 || value.length > 64 || !value.every(v => typeof v === 'string'))
        return undefined;
    const args = value;
    const shell = args[0].split('/').at(-1);
    if (args.length === 3 && ['sh', 'bash', 'zsh', 'dash', 'ksh'].includes(shell || '') && ['-c', '-lc'].includes(args[1]))
        return compactPreview(args[2]);
    // A display argv is not a shell command. Quote individual arguments so spaces
    // and metacharacters are not presented as a different invocation.
    return compactPreview(args.map(v => /^[a-zA-Z0-9_./:@%+=,-]+$/.test(v) ? v : JSON.stringify(v)).join(' '));
}
function activityPreview(step, lang = 'en') {
    const operation = activityOperation(step);
    if (operation === 'command' || ['read', 'search', 'list'].includes(operation)) {
        // A parsed file/query summary is more useful than the transport argv.
        const detail = step.detail?.trim();
        const transportSummary = detail && detail.length >= 16 && step.input?.startsWith(detail.replace(/…$/, ''));
        if (!detail || transportSummary || detail === step.input) {
            const command = commandPreview(step);
            if (command)
                return command;
        }
    }
    const structured = step.input_kind === 'parameters' ||
        step.input_kind === 'arguments' && (operation === 'agent' || operation === 'wait');
    if (structured && step.input) {
        if (step.detail && !step.detail.trimStart().startsWith('{') && step.detail !== step.title)
            return compactPreview(step.detail);
        let args = {};
        // Current cores bound each input to 8 KiB. Older/foreign inputs need no large parse.
        if (step.input.length <= 8192) {
            try {
                const parsed = JSON.parse(step.input);
                if (parsed && typeof parsed === 'object' && !Array.isArray(parsed))
                    args = parsed;
            }
            catch { /* A partial record stays available in the inspector. */ }
        }
        if (operation === 'agent') {
            for (const key of ['description', 'title', 'message', 'prompt', 'task_name']) {
                const value = args[key];
                if (typeof value === 'string' && value.trim())
                    return compactPreview(value);
            }
        }
        else if (operation === 'wait') {
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

var paths={
  read: 'M12 5C9 3 5 3 3 4v15c3-1 6-1 9 1 3-2 6-2 9-1V4c-2-1-6-1-9 1v15 M6 8h3 M15 8h3 M6 12h3 M15 12h3',
  list: 'M3 7V5h6l2 2h10v13H3V7Z M3 10h18',
  search: 'M16 10a6 6 0 1 1-12 0 6 6 0 0 1 12 0Z M14.5 14.5 21 21',
  command: 'M3 4h18v16H3Z M7 9l3 3-3 3 M13 15h4',
  edit: 'm14 5 5 5 M5 14l11-11 5 5-11 11-6 1 1-6Z',
  web: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z M3 12h18 M12 3c5 5 5 13 0 18-5-5-5-13 0-18Z',
  code: 'm8 6-6 6 6 6 M16 6l6 6-6 6 M14 3l-4 18',
  wait: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z M12 6v6l4 2',
  agent: 'M14 7a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z M4 21v-3a6 6 0 0 1 10-4 M16 17h6 M19 14l3 3-3 3',
  tool: 'M14 5a6 6 0 0 0-7 7l-5 5 5 5 5-5a6 6 0 0 0 7-7l-4 3-4-4 3-4Z',
};var api={operation:exports.activityOperation,label:exports.activityLabel,summary:exports.activitySummary,preview:exports.activityPreview,names:names,labels:labels,paths:paths};if(typeof module==="object"&&module.exports)module.exports=api;else root.GtmuxActivity=api;})(typeof window!=="undefined"?window:globalThis);
