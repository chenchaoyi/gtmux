// Generated from mobileapp/src/trace/model.ts; SHA256 a3b1f56163fe11a58d15ce7c38e896adbf9f07a8fda845415d81b92b33c04104
// Body SHA256 a0e9c68b23a279d9bbf7f07947790708770afcb8efa1de95715b043d5b973809
(function(root){var exports={};
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.agentName = agentName;
exports.build = build;
exports.filter = filter;
exports.overview = overview;
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
        const ordinal = (turnCounts.get(scope) || 0) + 1;
        turnCounts.set(scope, ordinal);
        const prompt = string(turn.prompt), time = string(turn.time), identity = scope + '\x00' + time + '\x00' + prompt;
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
        function push(kind, position, body, title, step) {
            const event = {
                key: key + ':' + position,
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
                sourceIndex,
                search: [title, body, agent, session, step?.workdir, step?.operation]
                    .filter(Boolean)
                    .join('\n')
                    .toLocaleLowerCase(),
            };
            events.push(event);
            sessionGroup.count++;
        }
        if (prompt)
            push('user', 'prompt', prompt, string(sender?.label), null);
        let segments = list(turn.segments);
        if (!segments.length && string(turn.response))
            segments = [{ text: turn.response }];
        segments.forEach((rawSegment, index) => {
            const segment = object(rawSegment);
            if (!segment)
                return;
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

if(typeof module==="object"&&module.exports)module.exports=exports;else root.GtmuxTrace=exports;})(typeof window!=="undefined"?window:globalThis);
