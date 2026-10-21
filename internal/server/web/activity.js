// Browser presentation of recorded activity. Keep semantics and icons aligned with
// mobileapp/src/ui/{activity.ts,ActivityIcon.tsx}; conformance tests pin their tables.
(function (root) {
  'use strict';
  var names = {
  read: 'read', readfile: 'read', read_file: 'read', read_text_file: 'read',
  ls: 'list', list: 'list', list_files: 'list', list_directory: 'list', glob: 'list',
  grep: 'search', rg: 'search', search: 'search', search_query: 'search', find: 'search', find_files: 'search',
  bash: 'command', shell: 'command', exec: 'command', exec_command: 'command', local_shell: 'command', run_command: 'command',
  edit: 'edit', write: 'edit', apply_patch: 'edit', patch: 'edit', write_file: 'edit', edit_file: 'edit',
  web_search: 'web', webfetch: 'web', web_fetch: 'web', browse: 'web', open_url: 'web', fetch: 'web',
  js: 'code', python: 'code', wait: 'wait', wait_for: 'wait', waitfor: 'wait', sleep: 'wait', wait_agent: 'wait', taskoutput: 'wait',
  agent: 'agent', task: 'agent', spawn_agent: 'agent', delegate: 'agent', followup_task: 'agent', resume_agent: 'agent',
};
  var labels = {
  read: ['Read file', '读取文件'], list: ['List files', '浏览目录'], search: ['Search', '搜索'],
  command: ['Run command', '运行命令'], edit: ['Edit file', '编辑文件'], web: ['Browse web', '访问网页'],
  code: ['Run code', '运行代码'], wait: ['Wait', '等待'], agent: ['Delegate', '委派任务'], tool: ['Tool call', '工具调用'],
};
  var paths = {
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
};
  function operation(step) {
    if (Object.prototype.hasOwnProperty.call(labels, step.operation)) return step.operation;
    var name = (step.title || '').split(/__|\./).pop().toLowerCase();
    return Object.prototype.hasOwnProperty.call(names, name) ? names[name] : 'tool';
  }
  function label(op, lang) { return labels[op][lang === 'zh' ? 1 : 0]; }
  function summary(steps, lang) {
    var ops = new Set(steps.map(operation));
    return ops.size === 1 ? label(ops.values().next().value, lang) : lang === 'zh' ? '执行过程' : 'Activity';
  }
  function compact(value) {
    var text = value.replace(/\s+/g, ' ').trim();
    return text.length <= 160 ? text : text.slice(0, 159).replace(/[\uD800-\uDBFF]$/, '') + '…';
  }
  function preview(step, lang) {
    var op = operation(step);
    var structured = step.input_kind === 'parameters' || step.input_kind === 'arguments' && (op === 'agent' || op === 'wait');
    if (structured && step.input) {
      if (step.detail && !step.detail.trimStart().startsWith('{') && step.detail !== step.title) return compact(step.detail);
      var args = {};
      if (step.input.length <= 8192) {
        try { var parsed = JSON.parse(step.input); if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) args = parsed; }
        catch (e) { /* Partial records stay available unchanged in the inspector. */ }
      }
      if (op === 'agent') {
        var keys = ['description', 'title', 'message', 'prompt', 'task_name'];
        for (var i = 0; i < keys.length; i++) {
          var purpose = args[keys[i]];
          if (typeof purpose === 'string' && purpose.trim()) return compact(purpose);
        }
      } else if (op === 'wait') {
        var durations = [['duration_ms', 'Requested', '设定'], ['yield_time_ms', 'Up to', '最多'], ['timeout_ms', 'Timeout setting', '超时设置']];
        for (var j = 0; j < durations.length; j++) {
          var d = durations[j], value = args[d[0]];
          if (typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= Number.MAX_SAFE_INTEGER)
            return lang === 'zh' ? d[2] + ' ' + value / 1000 + ' 秒' : d[1] + ' ' + value / 1000 + 's';
        }
      }
      return lang === 'zh' ? '已记录参数' : 'Recorded parameters';
    }
    return compact(step.detail || step.input || step.title || 'tool');
  }
  var api = {operation: operation, label: label, summary: summary, preview: preview, paths: paths, names: names, labels: labels};
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.GtmuxActivity = api;
})(typeof window === 'undefined' ? this : window);
