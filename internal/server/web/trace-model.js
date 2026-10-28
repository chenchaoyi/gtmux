/* Public transcript data only. No provider log, reasoning or execution-state inference. */
(function (root, factory) {
  if (typeof module === 'object' && module.exports) module.exports = factory();
  else root.GtmuxTrace = factory();
})(typeof window !== 'undefined' ? window : this, function () {
  'use strict';
  function string(value) { return typeof value === 'string' ? value : ''; }
  function list(value) { return Array.isArray(value) ? value : []; }
  function digest(value) {
    var a = 2166136261, b = 5381;
    for (var i = 0; i < value.length; i++) { a = Math.imul(a ^ value.charCodeAt(i), 16777619); b = Math.imul(b, 33) ^ value.charCodeAt(i); }
    return (a >>> 0).toString(36) + '-' + (b >>> 0).toString(36);
  }
  function agentName(value) {
    var name = string(value).trim();
    return ({codex: 'Codex', claude: 'Claude Code', 'claude code': 'Claude Code', dsh: 'DeepSeek Harness', 'deepseek harness': 'DeepSeek Harness', kimi: 'Kimi', opencode: 'OpenCode'})[name.toLowerCase()] || name;
  }
  function build(turns, context) {
    context = context || {};
    var events = [], sessions = [], groups = new Map(), occurrences = new Map(), turnCounts = new Map();
    list(turns).forEach(function (turn) {
      if (!turn || typeof turn !== 'object') return;
      var session = string(turn.session_id) || string(context.session_id) || string(context.pane_id) || 'current';
      var scope = string(context.home_id) + '\x00' + session;
      var group = groups.get(scope), agent = agentName(turn.agent || context.agent);
      if (!group) { group = {key: digest(scope), session: session, agent: agent, count: 0}; groups.set(scope, group); sessions.push(group); }
      var ordinal = (turnCounts.get(scope) || 0) + 1; turnCounts.set(scope, ordinal);
      var prompt = string(turn.prompt), time = string(turn.time), identity = scope + '\x00' + time + '\x00' + prompt;
      var occurrence = occurrences.get(identity) || 0; occurrences.set(identity, occurrence + 1);
      var key = digest(identity) + '-' + occurrence;
      function push(kind, position, body, title, step) {
        var event = {key: key + ':' + position, turn: ordinal, session: session, sessionKey: group.key, agent: agent,
          kind: kind, title: title, body: body, time: time, from: kind === 'user' ? turn.from || null : null,
          boundary: turn.session_break || null, step: step || null};
        event.search = [title, body, agent, session, step && step.workdir, step && step.operation].filter(Boolean).join('\n').toLocaleLowerCase();
        events.push(event); group.count++;
      }
      if (prompt) push('user', 'prompt', prompt, string(turn.from && turn.from.label), null);
      var segments = list(turn.segments);
      if (!segments.length && string(turn.response)) segments = [{text: turn.response}];
      segments.forEach(function (segment, index) {
        if (!segment || typeof segment !== 'object') return;
        if (string(segment.text)) push('assistant', index + ':text', segment.text, agent, null);
        list(segment.steps).forEach(function (step, offset) {
          if (!step || typeof step !== 'object') return;
          var input = string(step.input), detail = string(step.detail);
          push('tool', index + ':tool:' + offset, input || detail, string(step.title), {
            summary: detail, input: input, inputKind: string(step.input_kind), workdir: string(step.workdir),
            operation: string(step.operation), truncated: step.details_truncated === true,
          });
        });
      });
    });
    return {events: events, sessions: sessions, turns: Array.from(turnCounts.values()).reduce(function (a, b) { return a + b; }, 0)};
  }
  function filter(events, query, kind, session) {
    query = string(query).trim().toLocaleLowerCase();
    return list(events).filter(function (event) { return (!kind || kind === 'all' || event.kind === kind) && (!session || session === 'all' || event.sessionKey === session) && (!query || event.search.includes(query)); });
  }
  // Buckets cover the complete filtered sequence, including its final event.
  function overview(events, maximum) {
    var total = events.length, size = Math.max(1, Math.min(240, maximum || 120)), out = [];
    for (var i = 0, count = Math.min(total, size); i < count; i++) {
      var from = Math.floor(i * total / count), to = Math.floor((i + 1) * total / count), kind = events[from].kind;
      for (var j = from + 1; j < to; j++) if (events[j].kind !== kind) { kind = 'mixed'; break; }
      out.push({from: from, to: to, kind: kind});
    }
    return out;
  }
  return {build: build, filter: filter, overview: overview, agentName: agentName};
});
