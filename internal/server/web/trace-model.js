// Generated from shared App trace analysis and presentation; SHA256 b046f08b82df4b37b6d61379dadac5b43e5637768a8484b63a0ab63123e18d33
// Body SHA256 c8a8d93a1197d9c7e671e396237b0f68a3cce6d01c4452c40bd08cb088b3b2c6
(function(root){var modules={"model":function(exports,require){
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.turnBookmarkKey = turnBookmarkKey;
exports.agentName = agentName;
exports.build = build;
exports.sourceInfo = sourceInfo;
exports.contextLabel = contextLabel;
exports.overviewFacts = overviewFacts;
exports.groupTurns = groupTurns;
exports.filter = filter;
exports.overview = overview;
exports.timelineRows = timelineRows;
exports.previewText = previewText;
exports.mergeHistory = mergeHistory;
exports.prependHistory = prependHistory;
const toolPresentation_1 = require("../ui/toolPresentation");
const string = (value) => typeof value === 'string' ? value : '';
const list = (value) => (Array.isArray(value) ? value : []);
function object(value) {
    return value !== null && typeof value === 'object'
        ? value
        : null;
}
/* eslint-disable no-bitwise -- Stable identity uses two explicit 32-bit hashes. */
function digest(value) {
    let a = 2166136261, b = 5381;
    for (let i = 0; i < value.length; i++) {
        a = Math.imul(a ^ value.charCodeAt(i), 16777619);
        b = Math.imul(b, 33) ^ value.charCodeAt(i);
    }
    return (a >>> 0).toString(36) + '-' + (b >>> 0).toString(36);
}
/* eslint-enable no-bitwise */
// Persist only a real session and source identity, never a loaded row index or
// streaming reply body. Legacy records without either remain navigable, unmarked.
function turnBookmarkKey(value, context = {}) {
    const turn = object(value);
    if (!turn)
        return undefined;
    const session = string(turn.session_id) || string(context.session_id);
    const record = string(turn.record_id), time = string(turn.time);
    const ordinal = Number.isSafeInteger(turn.turn_ordinal) && turn.turn_ordinal > 0
        ? turn.turn_ordinal
        : null;
    if (!session || (!record && !time && !ordinal))
        return undefined;
    return digest(JSON.stringify([
        string(context.home_id),
        session,
        record
            ? ['record', record]
            : time
                ? ['time', time]
                : ['ordinal', ordinal],
    ]));
}
function agentName(value) {
    const name = string(value).trim();
    const brands = {
        codex: 'Codex',
        claude: 'Claude Code',
        'claude code': 'Claude Code',
        dsh: 'DeepSeek Harness',
        'deepseek harness': 'DeepSeek Harness',
        kimi: 'Kimi',
        opencode: 'OpenCode',
    };
    return Object.prototype.hasOwnProperty.call(brands, name.toLowerCase())
        ? brands[name.toLowerCase()]
        : name;
}
function build(turns, context = {}) {
    const ctx = context || {};
    const events = [], sessions = [];
    const groups = new Map(), occurrences = new Map(), turnCounts = new Map();
    // Old servers can omit both clocks and record IDs. Only ambiguous prompts
    // need a content fallback; a single streaming turn must retain its identity.
    const clockless = new Map();
    list(turns).forEach(raw => {
        const turn = object(raw);
        if (!turn || string(turn.record_id) || string(turn.time))
            return;
        const scope = string(ctx.home_id) +
            '\x00' +
            (string(turn.session_id) ||
                string(ctx.session_id) ||
                string(ctx.pane_id) ||
                'current');
        const value = scope + '\x00' + string(turn.prompt);
        clockless.set(value, (clockless.get(value) || 0) + 1);
    });
    list(turns).forEach((raw, sourceIndex) => {
        const turn = object(raw);
        if (!turn)
            return;
        const session = string(turn.session_id) ||
            string(ctx.session_id) ||
            string(ctx.pane_id) ||
            'current';
        const scope = string(ctx.home_id) + '\x00' + session, agent = agentName(turn.agent || ctx.agent);
        let group = groups.get(scope);
        if (!group) {
            group = { key: digest(scope), session, agent, count: 0 };
            groups.set(scope, group);
            sessions.push(group);
        }
        const sessionGroup = group;
        const ordinal = Number.isSafeInteger(turn.turn_ordinal) &&
            turn.turn_ordinal > 0
            ? turn.turn_ordinal
            : (turnCounts.get(scope) || 0) + 1;
        turnCounts.set(scope, ordinal);
        const prompt = string(turn.prompt), time = string(turn.time), identity = scope +
            '\x00' +
            (string(turn.record_id) || time) +
            '\x00' +
            prompt +
            (!string(turn.record_id) &&
                !time &&
                (clockless.get(scope + '\x00' + prompt) || 0) > 1
                ? '\x00' + JSON.stringify([turn.response, turn.segments])
                : '');
        const occurrence = occurrences.get(identity) || 0;
        occurrences.set(identity, occurrence + 1);
        const key = digest(identity) + '-' + occurrence;
        const sender = object(turn.from);
        const from = sender && (sender.kind === 'hq' || sender.kind === 'agent')
            ? {
                kind: sender.kind,
                label: string(sender.label),
                agent: string(sender.agent),
                pane: string(sender.pane),
            }
            : null;
        function push(kind, position, body, title, step, contextRecord = null) {
            const event = {
                key: key + ':' + position,
                turnKey: key,
                turn: ordinal,
                session,
                sessionKey: sessionGroup.key,
                agent,
                kind,
                title,
                body,
                time,
                from: kind === 'user' ? from : null,
                boundary: object(turn.session_break) ||
                    null,
                step,
                context: contextRecord,
                sourceIndex,
                search: [
                    title,
                    body,
                    agent,
                    session,
                    step?.workdir,
                    step?.operation,
                    contextRecord?.source,
                ]
                    .filter(Boolean)
                    .join('\n')
                    .toLocaleLowerCase(),
            };
            events.push(event);
            sessionGroup.count++;
        }
        let segments = list(turn.segments);
        if (!segments.length && string(turn.response))
            segments = [{ text: turn.response }];
        function pushContext(rawSegment, index, before) {
            const contextRecord = object(object(rawSegment)?.context);
            if (contextRecord &&
                (contextRecord.before_prompt === true) === before &&
                string(contextRecord.text))
                push('context', index + ':context', string(contextRecord.text), string(contextRecord.title), null, {
                    source: string(contextRecord.source),
                    time: string(contextRecord.time),
                    truncated: contextRecord.truncated === true,
                });
        }
        segments.forEach((segment, index) => pushContext(segment, index, true));
        if (prompt)
            push('user', 'prompt', prompt, string(sender?.label), null);
        segments.forEach((rawSegment, index) => {
            const segment = object(rawSegment);
            if (!segment)
                return;
            pushContext(segment, index, false);
            if (string(segment.text))
                push('assistant', index + ':text', string(segment.text), agent, null);
            list(segment.steps).forEach((rawStep, offset) => {
                const step = object(rawStep);
                if (!step)
                    return;
                const input = string(step.input), detail = string(step.detail);
                push('tool', index + ':tool:' + offset, input || detail, string(step.title), {
                    summary: detail,
                    input,
                    inputKind: string(step.input_kind),
                    workdir: string(step.workdir),
                    operation: string(step.operation),
                    action: string(step.action).slice(0, 32),
                    subjectCount: typeof step.subject_count === 'number' && Number.isSafeInteger(step.subject_count) && step.subject_count >= 1 && step.subject_count <= 10000 ? step.subject_count : undefined,
                    truncated: step.details_truncated === true,
                    inputRef: string(step.input_ref),
                    call: object(step.call),
                });
            });
        });
    });
    return {
        events,
        sessions,
        turns: new Set(events.map(event => event.turnKey)).size,
    };
}
/** Recorded provenance only. This is not the provider's original request JSON. */
function sourceInfo(event) {
    return {
        format: 'gtmux transcript',
        kind: event.kind,
        agent: event.agent,
        session_id: event.session,
        turn: event.turn,
        record_source: event.context?.source ||
            (event.kind === 'context'
                ? 'segments.context'
                : event.kind === 'tool'
                    ? 'segments.steps'
                    : event.kind === 'user'
                        ? 'prompt'
                        : 'segments.text'),
        ...(event.time ? { turn_started_at: event.time } : {}),
        ...(event.context?.time ? { recorded_at: event.context.time } : {}),
        ...(event.from ? { sender: event.from } : {}),
        ...(event.step?.workdir ? { workdir: event.step.workdir } : {}),
        ...(event.step
            ? {
                input_kind: event.step.inputKind,
                operation: event.step.operation,
                ...(event.step.action ? { action: event.step.action } : {}),
                ...(event.step.subjectCount ? { subject_count: event.step.subjectCount } : {}),
                truncated: event.step.truncated,
            }
            : {}),
        ...(event.context ? { truncated: event.context.truncated } : {}),
    };
}
function contextLabel(event, lang) {
    const names = {
        'Workspace instructions': ['Workspace instructions', '工作区指令'],
        Environment: ['Environment', '运行环境'],
        'Environment reminder': ['Environment reminder', '环境提醒'],
        'Background task notification': [
            'Background task notification',
            '后台任务通知',
        ],
        'Conversation summary': ['Conversation summary', '会话摘要'],
        'Harness input': ['Context', '上下文'],
    };
    return (names[event.title]?.[lang === 'zh' ? 1 : 0] ||
        event.title ||
        (lang === 'zh' ? '上下文' : 'Context'));
}
function overviewFacts(event, lang) {
    const zh = lang === 'zh', kinds = {
        user: ['Message', '用户消息'],
        assistant: ['Agent reply', 'Agent 回复'],
        tool: ['Tool call', '工具调用'],
        context: ['Context', '上下文'],
    };
    const facts = [
        { label: zh ? '类型' : 'Type', value: kinds[event.kind][zh ? 1 : 0] },
        { label: zh ? '轮次' : 'Turn', value: String(event.turn) },
        { label: 'Agent', value: event.agent },
        { label: zh ? '会话' : 'Session', value: event.session },
    ];
    if (event.step?.call)
        facts.unshift(...(0, toolPresentation_1.callFacts)({ kind: 'tool', title: event.title, call: event.step.call }, lang));
    if (event.context?.time)
        facts.push({
            label: zh ? '记录时间' : 'Recorded at',
            value: event.context.time,
        });
    if (event.step?.workdir)
        facts.push({
            label: zh ? '工作目录' : 'Working directory',
            value: event.step.workdir,
        });
    return facts.filter(fact => fact.value);
}
/** Group only recorded order and identity; no outcome or timing is inferred. */
function groupTurns(events) {
    const groups = new Map();
    for (const event of events) {
        let group = groups.get(event.turnKey);
        if (!group) {
            group = {
                key: event.turnKey,
                sessionKey: event.sessionKey,
                session: event.session,
                agent: event.agent,
                turn: event.turn,
                time: event.time,
                prompt: null,
                events: [],
                calls: 0,
            };
            groups.set(event.turnKey, group);
        }
        group.events.push(event);
        if (event.kind === 'user')
            group.prompt = event;
        if (event.kind === 'tool')
            group.calls++;
    }
    return Array.from(groups.values());
}
function filter(events, query = '', kind = 'all', session = 'all') {
    const needle = string(query).trim().toLocaleLowerCase();
    return events.filter(event => (!kind || kind === 'all' || event.kind === kind) &&
        (!session || session === 'all' || event.sessionKey === session) &&
        (!needle || event.search.includes(needle)));
}
function overview(events, maximum = 120) {
    const total = events.length, size = Math.max(1, Math.min(240, maximum || 120)), out = [];
    for (let i = 0, count = Math.min(total, size); i < count; i++) {
        const from = Math.floor((i * total) / count), to = Math.floor(((i + 1) * total) / count);
        let kind = events[from].kind;
        for (let j = from + 1; j < to; j++)
            if (events[j].kind !== kind) {
                kind = 'mixed';
                break;
            }
        out.push({ from, to, kind });
    }
    return out;
}
function timelineRows(groups, expanded, calls, searching = false, defaults = {}) {
    const rows = [];
    for (const group of groups) {
        const open = searching || (expanded[group.key] ?? defaults.turns ?? false);
        rows.push({ key: 'turn:' + group.key, type: 'turn', group, open });
        if (!open)
            continue;
        for (let i = 0; i < group.events.length; i++) {
            const event = group.events[i];
            let end = i + 1;
            if (!searching && event.kind === 'tool') {
                while (end < group.events.length && group.events[end].kind === 'tool')
                    end++;
            }
            if (!searching &&
                event.kind === 'tool' &&
                end - i >= (defaults.calls === undefined ? 3 : 1)) {
                const key = 'calls:' + event.key, unfolded = calls[key] ?? defaults.calls ?? false;
                rows.push({
                    key,
                    type: 'calls',
                    events: group.events.slice(i, end),
                    open: unfolded,
                    last: end === group.events.length && !unfolded,
                });
                if (!unfolded) {
                    i = end - 1;
                    continue;
                }
                for (let j = i; j < end; j++)
                    rows.push({
                        key: group.events[j].key,
                        type: 'event',
                        event: group.events[j],
                        order: j + 1,
                        last: j === group.events.length - 1,
                        parentCall: { key, index: j - i + 1, total: end - i },
                    });
                i = end - 1;
            }
            else
                rows.push({
                    key: event.key,
                    type: 'event',
                    event,
                    order: i + 1,
                    last: i === group.events.length - 1,
                });
        }
    }
    return rows;
}
/** Reading transforms only; raw content and copy retain the recorded text. */
function previewText(event) {
    if (event.kind === 'tool' &&
        ['json', 'arguments', 'changes'].includes(event.step?.inputKind || '')) {
        try {
            const value = JSON.parse(event.body);
            if (value !== null && typeof value === 'object')
                return JSON.stringify(value, null, 2);
        }
        catch {
            /* Older records may contain partial JSON. */
        }
    }
    if (event.kind === 'context') {
        const envelope = /^\s*<(system-reminder|task-notification)\b[^>]*>([\s\S]*)<\/\1>\s*$/.exec(event.body);
        if (envelope)
            return envelope[2].trim();
    }
    return event.body;
}
// Keep explicitly opened history across tail polls. A missing overlap is visible;
// never join an old window to a new tail across an unobserved gap or source reset.
function mergeHistory(held, latest) {
    if (!held.length)
        return { turns: latest, gap: false };
    if (!latest.length)
        return { turns: held, gap: false };
    const key = (t) => t.record_id
        ? `${t.session_id || ''}\x00${t.record_id}\x00${t.turn_ordinal || ''}\x00${digest((t.time || '') + '\x00' + t.prompt)}`
        : '';
    const keys = new Set(latest.map(key).filter(Boolean));
    const overlap = held.findIndex(t => keys.has(key(t)));
    if (overlap >= 0)
        return { turns: held.slice(0, overlap).concat(latest), gap: false };
    const end = held[held.length - 1], start = latest[0];
    if (end.session_id === start.session_id &&
        end.turn_ordinal &&
        start.turn_ordinal === end.turn_ordinal + 1) {
        return { turns: held.concat(latest), gap: false };
    }
    return { turns: held, gap: true };
}
function prependHistory(page, current) {
    const end = page[page.length - 1], start = current[0];
    if (!end ||
        !start ||
        end.session_id !== start.session_id ||
        !end.turn_ordinal ||
        start.turn_ordinal !== end.turn_ordinal + 1)
        throw new Error('History anchor changed');
    return page.concat(current);
}

},
"activity":function(exports,require){
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.activityOperation = activityOperation;
exports.activityLabel = activityLabel;
exports.activityActionLabel = activityActionLabel;
exports.activitySummary = activitySummary;
exports.activityPreview = activityPreview;
const names = {
    read: 'read', readfile: 'read', read_file: 'read', read_text_file: 'read',
    ls: 'list', list: 'list', list_files: 'list', list_directory: 'list', glob: 'list',
    grep: 'search', rg: 'search', search: 'search', search_query: 'search', find: 'search', find_files: 'search',
    bash: 'command', shell: 'command', exec: 'command', exec_command: 'command', local_shell: 'command', run_command: 'command',
    edit: 'edit', write: 'edit', apply_patch: 'edit', patch: 'edit', write_file: 'edit', edit_file: 'edit',
    web_search: 'web', websearch: 'web', webfetch: 'web', web_fetch: 'web', browse: 'web', open_url: 'web', fetch: 'web',
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
    if (step.title === 'web.run' || step.title === 'web__run')
        return 'web';
    const name = (step.title || '').split(/__|\./).pop()?.toLowerCase() || '';
    return names[name] || 'tool';
}
function activityLabel(operation, lang) {
    return labels[operation][lang === 'zh' ? 1 : 0];
}
// Action/count refine recorded facts; they never override a contradictory base
// operation or turn a single multi-target invocation into several tool calls.
function activityActionLabel(step, lang) {
    const operation = activityOperation(step), zh = lang === 'zh';
    const count = typeof step.subject_count === 'number' && Number.isSafeInteger(step.subject_count) && step.subject_count >= 1 && step.subject_count <= 10000 ? step.subject_count : 0;
    if (step.action === 'skill' && (operation === 'read' || operation === 'tool') && count <= 1)
        return zh ? '读取技能' : 'Read skill';
    if (operation === 'read' && count > 1)
        return zh ? `读取 ${count} 个文件` : `Read ${count} files`;
    if (operation !== 'web')
        return undefined;
    const method = (step.title || '').split(/__|\./).pop()?.toLowerCase();
    const action = step.action || (['websearch', 'web_search'].includes(method || '') ? 'web_search' : ['webfetch', 'web_fetch', 'open_url', 'fetchurl'].includes(method || '') ? 'web_open' : '');
    if (action === 'web_search')
        return zh ? '搜索网页' : 'Search web';
    if (action === 'web_open')
        return zh ? '打开网页' : 'Open webpage';
    if (action === 'web_find')
        return zh ? '页内查找' : 'Find in page';
    return undefined;
}
function activitySummary(steps, lang) {
    const titles = [...new Set(steps.map(step => activityActionLabel(step, lang) || activityLabel(activityOperation(step), lang)))];
    if (titles.length === 1) {
        return titles[0];
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

},
"timeline":function(exports,require){
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.timelineRows = void 0;
exports.tracePreviewSource = tracePreviewSource;
exports.tracePreview = tracePreview;
var model_1 = require("./model");
Object.defineProperty(exports, "timelineRows", { enumerable: true, get: function () { return model_1.timelineRows; } });
// Preserve Markdown and line breaks for a rich preview. The compact trace row
// still uses tracePreview below; image transport envelopes never become prose.
function tracePreviewSource(value, lang) {
    let images = 0;
    const clean = value.replace(/<image\s+name=[^>]*\bpath="[^"]+"[^>]*>/g, () => {
        images++;
        return '';
    });
    const text = (images ? clean.replace(/<\/image>/g, '') : clean).trim();
    return (text || (images ? (lang === 'zh' ? '图片消息' : 'Image message') : ''));
}
function tracePreview(value, lang) {
    return tracePreviewSource(value, lang).replace(/\s+/g, ' ').slice(0, 240);
}

},
"presentation":function(exports,require){
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.traceRecordPaths = exports.traceDetailLabels = exports.traceKindLabels = void 0;
exports.recordedStep = recordedStep;
exports.traceEventLabel = traceEventLabel;
exports.traceCounts = traceCounts;
exports.traceWindowLabel = traceWindowLabel;
exports.traceTurnSummary = traceTurnSummary;
exports.traceDetailMetadata = traceDetailMetadata;
exports.traceEventPreview = traceEventPreview;
exports.traceEventIcon = traceEventIcon;
exports.conversationOutline = conversationOutline;
const model_1 = require("./model");
const activity_1 = require("../ui/activity");
const toolPresentation_1 = require("../ui/toolPresentation");
const timeline_1 = require("./timeline");
exports.traceKindLabels = {
    all: ['All records', '全部记录'],
    user: ['Messages', '用户消息'],
    assistant: ['Agent replies', 'Agent 回复'],
    tool: ['Tool calls', '工具调用'],
    context: ['Context', '上下文'],
};
exports.traceDetailLabels = {
    overview: ['Overview', '概述'],
    preview: ['Preview', '预览'],
    raw: ['Raw content', '原始内容'],
    origin: ['Source', '来源'],
};
function recordedStep(event) {
    const step = event.step;
    return {
        kind: 'tool',
        call: step.call,
        workdir: step.workdir,
        input_ref: step.inputRef,
        details_truncated: step.truncated,
        title: event.title,
        input: step.input,
        detail: step.summary,
        operation: step.operation,
        action: step.action,
        subject_count: step.subjectCount,
        input_kind: step.inputKind,
    };
}
function traceEventLabel(event, lang) {
    if (event.kind === 'context')
        return (0, model_1.contextLabel)(event, lang);
    return event.kind === 'tool'
        ? (0, toolPresentation_1.callTitle)(recordedStep(event), lang) || (0, activity_1.activityLabel)((0, activity_1.activityOperation)(recordedStep(event)), lang)
        : event.kind === 'user'
            ? event.from?.label || (lang === 'zh' ? '请求' : 'Request')
            : lang === 'zh'
                ? 'Agent 回复'
                : 'Agent reply';
}
// Counts describe the loaded window, never a source ordinal or a whole session.
// Both renderers use these labels so context cannot be counted as a message.
function traceCounts(events) {
    return {
        turns: new Set(events.map(event => event.turnKey)).size,
        calls: events.filter(event => event.kind === 'tool').length,
        messages: events.filter(event => event.kind === 'user' || event.kind === 'assistant').length,
        contexts: events.filter(event => event.kind === 'context').length,
        records: events.length,
    };
}
function traceWindowLabel(events, lang) {
    const counts = traceCounts(events);
    return lang === 'zh'
        ? `${counts.turns} 轮 · ${counts.records} 条记录`
        : `${counts.turns} ${counts.turns === 1 ? 'turn' : 'turns'} · ${counts.records} ${counts.records === 1 ? 'record' : 'records'}`;
}
function traceTurnSummary(events, lang) {
    const counts = traceCounts(events);
    const parts = lang === 'zh'
        ? [`${counts.calls} 次调用`, `${counts.messages} 条消息`]
        : [
            `${counts.calls} ${counts.calls === 1 ? 'call' : 'calls'}`,
            `${counts.messages} ${counts.messages === 1 ? 'message' : 'messages'}`,
        ];
    if (counts.contexts) {
        parts.push(lang === 'zh'
            ? `${counts.contexts} 条上下文`
            : `${counts.contexts} context ${counts.contexts === 1 ? 'record' : 'records'}`);
    }
    return parts.join(' · ');
}
function traceDetailMetadata(event, lang, time = '') {
    return [
        event.agent,
        lang === 'zh' ? `第 ${event.turn} 轮` : `Turn ${event.turn}`,
        time ? (lang === 'zh' ? '本轮开始于 ' : 'Turn started ') + time : '',
    ]
        .filter(Boolean)
        .join(' · ');
}
function traceEventPreview(event, lang) {
    return event.kind === 'tool'
        ? (0, activity_1.activityPreview)(recordedStep(event), lang)
        : (0, timeline_1.tracePreview)(event.body, lang);
}
function traceEventIcon(event) {
    return event.kind === 'tool'
        ? (0, activity_1.activityOperation)(recordedStep(event))
        : event.kind;
}
// Same neutral glyphs for non-tool records in both renderers; sender identities
// use each client's existing shared avatar component when one was recorded.
exports.traceRecordPaths = {
    context: 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18 M12 11v6 M12 7v.1',
    user: 'M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0 M4 21v-2a8 8 0 0 1 16 0v2',
    assistant: 'm12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5L12 3Z',
};
// Loaded-window navigation reuses trace identity, original ordinals and summaries.
function conversationOutline(turns, lang = 'en', context) {
    return (0, model_1.groupTurns)((0, model_1.build)(turns, context).events).map(group => ({
        key: group.key,
        bookmarkKey: (0, model_1.turnBookmarkKey)(turns[group.events[0].sourceIndex], context),
        sourceIndex: group.events[0].sourceIndex,
        turn: group.turn,
        agent: group.agent,
        session: group.session,
        prompt: (0, timeline_1.tracePreview)(group.prompt?.body || group.events[0].body, lang),
        promptMarkdown: (0, timeline_1.tracePreviewSource)(group.prompt?.body || group.events[0].body, lang).slice(0, 8192),
        summary: (0, timeline_1.tracePreview)([...group.events].reverse().find(event => event.kind === 'assistant')
            ?.body || '', lang),
        summaryMarkdown: (0, timeline_1.tracePreviewSource)([...group.events].reverse().find(event => event.kind === 'assistant')
            ?.body || '', lang).slice(0, 8192),
    }));
}

},
"toolPresentation":function(exports,require){
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.callTitle = callTitle;
exports.callFacts = callFacts;
exports.callSections = callSections;
exports.callRaw = callRaw;
exports.toolDetailRefs = toolDetailRefs;
exports.loadToolDetails = loadToolDetails;
exports.callPage = callPage;
const activity_1 = require("./activity");
// Canonical Chat/Trace interpretation, also generated for the browser. These
// fields describe recorded evidence; never infer success from a tool name.
function callTitle(step, lang = 'en') {
    const actionLabel = (0, activity_1.activityActionLabel)(step, lang);
    if (actionLabel)
        return actionLabel;
    const operation = (0, activity_1.activityOperation)(step);
    if (operation !== 'tool')
        return (0, activity_1.activityLabel)(operation, lang);
    const call = step.call;
    if (!call)
        return step.title;
    const name = call.name || step.title;
    const action = name.split(/__|\./).at(-1) || name;
    const words = action.replace(/[_-]+/g, ' ');
    return [call.provider, words.charAt(0).toUpperCase() + words.slice(1)]
        .filter(Boolean)
        .join(' · ');
}
function callFacts(step, lang) {
    const c = step.call;
    if (!c)
        return [];
    const zh = lang === 'zh';
    const statuses = {
        completed: ['Completed', '已完成'],
        failed: ['Failed', '失败'],
        in_progress: ['Running', '进行中'],
        cancelled: ['Cancelled', '已取消'],
    };
    const facts = [{ label: zh ? '工具' : 'Tool', value: c.name || step.title }];
    if (c.provider)
        facts.push({ label: zh ? '服务' : 'Provider', value: c.provider });
    if (c.status)
        facts.push({
            label: zh ? '状态' : 'Status',
            value: statuses[c.status]?.[zh ? 1 : 0] || c.status,
        });
    if (typeof c.duration_ms === 'number')
        facts.push({
            label: zh ? '耗时' : 'Duration',
            value: c.duration_ms < 1000
                ? c.duration_ms + ' ms'
                : (c.duration_ms / 1000).toFixed(2) + ' s',
        });
    return facts;
}
function hasLossyJSONNumber(value) {
    // Scan JSON tokens linearly, with no decoded body or regex over large strings.
    // Preserve opaque 64-bit IDs and decimal values that Number would change.
    for (let i = 0; i < value.length; i++) {
        if (value[i] === '"') {
            for (i++; i < value.length; i++) {
                if (value[i] === '\\')
                    i++;
                else if (value[i] === '"')
                    break;
            }
            continue;
        }
        if (value[i] !== '-' && !(value[i] >= '0' && value[i] <= '9'))
            continue;
        const start = i;
        while (i + 1 < value.length && '0123456789.eE+-'.includes(value[i + 1]))
            i++;
        const token = value.slice(start, i + 1), n = Number(token);
        if (Object.is(n, -0) ||
            !Number.isFinite(n) ||
            (Number.isInteger(n) && !Number.isSafeInteger(n)))
            return true;
        const canonical = (text) => {
            const [mantissa, exp = '0'] = text.toLowerCase().split('e');
            const dot = mantissa.indexOf('.');
            let power = Number(exp) - (dot < 0 ? 0 : mantissa.length - dot - 1);
            let digits = mantissa.replace('.', '').replace(/^(-?)0+/, '$1');
            const trimmed = digits.replace(/0+$/, '');
            power += digits.length - trimmed.length;
            digits = trimmed;
            return digits === '' || digits === '-' ? '0' : digits + 'e' + power;
        };
        if (canonical(token) !== canonical(String(n)))
            return true;
    }
    return false;
}
function json(value) {
    if (hasLossyJSONNumber(value))
        return undefined;
    try {
        return JSON.parse(value);
    }
    catch {
        return undefined;
    }
}
function callSections(step, lang) {
    const zh = lang === 'zh', c = step.call, sections = [];
    const add = (id, label, source, hint = 'text', parameters = false) => {
        const value = source.length <= 16000 ? json(source) : undefined;
        const rows = parameters &&
            value &&
            typeof value === 'object' &&
            !Array.isArray(value) &&
            Object.keys(value).length <= 24
            ? Object.entries(value).map(([key, v]) => ({
                label: key.replace(/_/g, ' '),
                value: typeof v === 'string' ? v : JSON.stringify(v, null, 2),
            }))
            : undefined;
        sections.push({
            id,
            label,
            source,
            hint,
            rows,
            verbatim: hint === 'json' && hasLossyJSONNumber(source),
        });
    };
    const argv = step.input_kind === 'arguments' &&
        ['command', 'read', 'list', 'search'].includes(step.operation || '');
    if (step.input &&
        step.input_kind !== 'parameters' &&
        (step.input_kind !== 'arguments' || argv)) {
        add('input', argv
            ? zh
                ? '命令参数'
                : 'Command arguments'
            : step.input_kind === 'changes'
                ? zh
                    ? '文件变更'
                    : 'File changes'
                : step.input_kind === 'command'
                    ? zh
                        ? '命令'
                        : 'Command'
                    : step.input_kind === 'code'
                        ? zh
                            ? '代码'
                            : 'Code'
                        : zh
                            ? '输入'
                            : 'Input', step.input, argv
            ? 'json'
            : step.input_kind === 'code'
                ? 'javascript'
                : step.input_kind === 'command'
                    ? 'shell'
                    : step.input_kind === 'changes'
                        ? 'json'
                        : 'text');
    }
    const args = c?.arguments ||
        (['parameters', 'arguments'].includes(step.input_kind || '')
            ? step.input || ''
            : '');
    if (args &&
        !argv &&
        (!step.input ||
            ['parameters', 'arguments'].includes(step.input_kind || '') ||
            step.operation === 'tool'))
        add('arguments', zh ? '参数' : 'Parameters', args, 'json', true);
    if (c?.has_result && c.loaded) {
        const result = (c.result?.length || 0) < 262144 ? json(c.result || '') : undefined;
        // Prefer structured data, otherwise preserve every MCP text block. Raw keeps
        // the entire sanitized recorded envelope, including error/type metadata.
        if (result === null ||
            result === '' ||
            (Array.isArray(result?.content) &&
                result.content.length === 0 &&
                result.structuredContent == null))
            add('empty', zh ? '结果' : 'Result', zh ? '调用已返回，没有内容。' : 'The call returned no content.', 'notice');
        else if (result &&
            typeof result === 'object' &&
            result.structuredContent != null)
            add('result', zh ? '结果' : 'Result', JSON.stringify(result.structuredContent, null, 2), 'json');
        else if (result && Array.isArray(result.content)) {
            const blocks = result.content.filter((v) => !!v &&
                typeof v === 'object' &&
                v.type === 'text' &&
                typeof v.text === 'string');
            if (blocks.length)
                blocks.forEach((block, i) => add('result-' + i, zh ? '结果' : 'Result', block.text, json(block.text) !== undefined ? 'json' : 'text'));
            else
                add('result', zh ? '结果' : 'Result', c.result || 'null', 'json');
        }
        else
            add('result', zh ? '结果' : 'Result', typeof result === 'string' ? result : c.result || 'null', typeof result === 'string' ? 'text' : 'json');
        if (result?.isError === true)
            sections.unshift({
                id: 'error',
                label: zh ? '工具返回错误' : 'Tool returned an error',
                source: zh
                    ? '以下为工具记录的错误结果。'
                    : 'The tool recorded an error result.',
                hint: 'notice',
            });
    }
    if (c && !c.loaded && (c.has_arguments || c.has_result))
        add('pending', zh ? '详情' : 'Details', zh
            ? '打开详情以读取已记录的参数和结果。'
            : 'Open details to load recorded parameters and results.', 'notice');
    else if (c && !c.has_result)
        add('unrecorded', zh ? '结果' : 'Result', zh
            ? '此记录未包含工具结果。'
            : 'No tool result is included in this record.', 'notice');
    if (!sections.length)
        add('summary', zh ? '已记录摘要' : 'Recorded summary', step.detail ||
            (zh
                ? '此记录未包含详细输入。'
                : 'No detailed input is included in this record.'), 'notice');
    if (c?.redacted)
        add('redacted', zh ? '隐私保护' : 'Privacy', zh
            ? '凭据字段和二进制数据已隐藏。'
            : 'Credential fields and binary data are hidden.', 'notice');
    return sections;
}
function callRaw(step) {
    const c = step.call;
    if (!c?.loaded)
        return (step.input ||
            step.detail ||
            (c
                ? JSON.stringify({
                    tool: c.name || step.title,
                    status: c.status || '',
                    details: 'not loaded',
                }, null, 2)
                : ''));
    // Raw describes the sanitized recorded structure, not formatted command source.
    return ('{\n  "tool": ' +
        JSON.stringify(c.name || step.title) +
        ',\n  "status": ' +
        JSON.stringify(c.status || '') +
        (step.input ? ',\n  "input": ' + JSON.stringify(step.input) : '') +
        (c.has_arguments ? ',\n  "arguments": ' + (c.arguments || 'null') : '') +
        (c.has_result ? ',\n  "result": ' + (c.result || 'null') : '') +
        '\n}');
}
function toolDetailRefs(step) {
    return [
        ...new Set([
            step.details_truncated ? step.input_ref : '',
            step.call && !step.call.loaded ? step.call.ref : '',
        ].filter((v) => !!v)),
    ];
}
async function loadToolDetails(step, read) {
    const refs = toolDetailRefs(step), values = await Promise.all(refs.map(read));
    let merged = { ...step };
    for (let i = 0; i < refs.length; i++) {
        const value = values[i];
        if (value.input_ref !== refs[i] || value.details_truncated)
            throw new Error('Incomplete tool details');
        if (step.details_truncated && step.input_ref === refs[i])
            merged = {
                ...merged,
                input: value.input,
                workdir: value.workdir,
                details_truncated: false,
            };
        if (step.call?.ref === refs[i]) {
            if (!value.call?.loaded ||
                value.call.ref !== refs[i] ||
                (value.call.has_arguments &&
                    typeof value.call.arguments !== 'string') ||
                (value.call.has_result && typeof value.call.result !== 'string'))
                throw new Error('Incomplete recorded call');
            merged = { ...merged, call: value.call };
        }
    }
    return merged;
}
// Bound rendering cost, never the stored/copyable source. Page labels disclose
// the visual window; UTF-16 surrogate pairs are kept intact across boundaries.
function callPage(source, page) {
    const size = 16000, pages = Math.max(1, Math.ceil(source.length / size));
    const index = Math.max(0, Math.min(pages - 1, page));
    const boundary = (n) => n > 0 && /[\uDC00-\uDFFF]/.test(source.charAt(n)) ? n - 1 : n;
    return {
        source: source.slice(boundary(index * size), boundary(Math.min(source.length, (index + 1) * size))),
        page: index,
        pages,
    };
}

}},cache={};function load(key){if(cache[key])return cache[key];var exports={};cache[key]=exports;modules[key](exports,function(name){var id={"./model":"model","../ui/activity":"activity","./activity":"activity","./timeline":"timeline","../ui/toolPresentation":"toolPresentation"}[name];if(!id)throw new Error("Unknown shared trace module");return load(id);});return exports;}var api=Object.assign({},load("model"),load("presentation"),load("toolPresentation"));
if(typeof module==="object"&&module.exports)module.exports=api;else root.GtmuxTrace=api;})(typeof window!=="undefined"?window:globalThis);
