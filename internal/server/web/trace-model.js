// Generated from shared App trace analysis and presentation; SHA256 c631356902a55f8fdb3b5083d2bca68941c4f0f404006db25519dc3d7156ff35
// Body SHA256 e30dc55888bb8c5bdfd5ed667e6d5e0cbdb5b19d0c223ac70df4cf3e305c3923
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
    const ordinal = Number.isSafeInteger(turn.turn_ordinal) && turn.turn_ordinal > 0 ? turn.turn_ordinal : null;
    if (!session || !record && !time && !ordinal)
        return undefined;
    return digest(JSON.stringify([string(context.home_id), session, record ? ['record', record] : time ? ['time', time] : ['ordinal', ordinal]]));
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
                    truncated: step.details_truncated === true,
                    inputRef: string(step.input_ref),
                });
            });
        });
    });
    return {
        events,
        sessions,
        turns: Array.from(turnCounts.values()).reduce((a, b) => a + b, 0),
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
        ['json', 'arguments'].includes(event.step?.inputKind || '')) {
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
    const key = (t) => t.record_id ? `${t.session_id || ''}\x00${t.record_id}\x00${t.turn_ordinal || ''}\x00${digest((t.time || '') + '\x00' + t.prompt)}` : '';
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
function activityPreview(step, lang = 'en') {
    const operation = activityOperation(step);
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
exports.traceEventPreview = traceEventPreview;
exports.traceEventIcon = traceEventIcon;
exports.conversationOutline = conversationOutline;
const model_1 = require("./model");
const activity_1 = require("../ui/activity");
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
        title: event.title,
        input: step.input,
        detail: step.summary,
        operation: step.operation,
        input_kind: step.inputKind,
    };
}
function traceEventLabel(event, lang) {
    if (event.kind === 'context')
        return (0, model_1.contextLabel)(event, lang);
    return event.kind === 'tool'
        ? (0, activity_1.activityLabel)((0, activity_1.activityOperation)(recordedStep(event)), lang)
        : event.kind === 'user'
            ? event.from?.label || (lang === 'zh' ? '请求' : 'Request')
            : lang === 'zh'
                ? '进展记录'
                : 'Progress';
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

}},cache={};function load(key){if(cache[key])return cache[key];var exports={};cache[key]=exports;modules[key](exports,function(name){var id={"./model":"model","../ui/activity":"activity","./timeline":"timeline"}[name];if(!id)throw new Error("Unknown shared trace module");return load(id);});return exports;}var api=Object.assign({},load("model"),load("presentation"));
if(typeof module==="object"&&module.exports)module.exports=api;else root.GtmuxTrace=api;})(typeof window!=="undefined"?window:globalThis);
