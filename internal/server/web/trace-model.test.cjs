const {test} = require('node:test');
const assert = require('node:assert/strict');
const trace = require('./trace-model.js');

test('providers preserve text/call order and recorded input without duplicating response', () => {
  for (const agent of ['codex', 'claude', 'kimi', 'opencode', 'dsh']) {
    const result = trace.build([{session_id: 'one', prompt: 'Review', response: 'Combined', agent,
      segments: [{steps: [{title: 'call', detail: 'short', input: 'line one\n  line two', workdir: '/project', input_kind: 'command', operation: 'shell', details_truncated: true}]}, {text: 'Reply'}]}]);
    assert.deepEqual(result.events.map(e => e.kind), ['user', 'tool', 'assistant']);
    assert.equal(result.events[1].body, 'line one\n  line two');
    assert.equal(result.events[1].step.workdir, '/project');
    assert.equal(result.events[1].step.truncated, true);
    assert.equal(result.events[2].agent, trace.agentName(agent));
    assert.ok(!result.events.some(e => e.body === 'Combined'));
    assert.ok(!('duration' in result.events[1]) && !('output' in result.events[1]));
  }
});

test('identity survives older history and streaming growth', () => {
  const original = {session_id: 'current', time: '2026-10-28T10:00:00Z', prompt: 'same', segments: [{text: 'In progress'}]};
  const first = trace.build([original]).events;
  const next = trace.build([{...original, time: '2026-10-27T10:00:00Z'}, {...original, segments: [{text: 'More text'}, {steps: [{title: 'Read'}]}]}]).events;
  assert.equal(next[2].key, first[0].key); assert.equal(next[3].key, first[1].key);
  assert.equal(new Set(next.map(e => e.key)).size, next.length);
});

test('homes and mixed HQ histories stay distinct with per-session turn numbering', () => {
  const turns = [{session_id: 'old', agent: 'claude', prompt: 'Before'}, {session_id: 'new', agent: 'codex', prompt: 'After', session_break: {kind: 'new'}}];
  const result = trace.build(turns, {home_id: 'home-a'});
  assert.equal(result.sessions.length, 2);
  assert.deepEqual(result.events.map(e => [e.session, e.agent, e.turn]), [['old', 'Claude Code', 1], ['new', 'Codex', 1]]);
  assert.equal(result.events[1].boundary.kind, 'new');
  assert.notEqual(result.events[0].key, trace.build(turns, {home_id: 'home-b'}).events[0].key);
});

test('missing time, identical prompts and legacy responses stay readable and unique', () => {
  const result = trace.build([{prompt: 'same', response: 'first'}, {prompt: 'same', response: 'second'}, {segments: [{steps: [{title: 'tool', detail: 'summary'}]}]}], {agent: 'kimi', pane_id: '%7'});
  assert.equal(result.events.length, 5); assert.equal(result.events[4].body, 'summary');
  assert.equal(result.events[4].time, ''); assert.equal(new Set(result.events.map(e => e.key)).size, 5);
});

test('search covers full input, workdir, agent and session; filters compose', () => {
  const result = trace.build([{session_id: 'review-session', agent: 'dsh', prompt: 'Review', segments: [{steps: [{title: 'bash', detail: 'summary', input: 'echo 中文', workdir: '/long/project'}]}]}]);
  for (const query of ['中文', '/long/project', 'deepseek', 'review-session']) assert.equal(trace.filter(result.events, query, 'tool', 'all').length, 1);
  assert.equal(trace.filter(result.events, '', 'tool', 'foreign').length, 0);
  assert.equal(trace.filter(result.events, 'absent', 'all', 'all').length, 0);
});

test('bounded overview covers every event, including the last of a long history', () => {
  const events = Array.from({length: 10001}, (_, i) => ({kind: i === 10000 ? 'tool' : 'assistant'}));
  const buckets = trace.overview(events, 120);
  assert.equal(buckets.length, 120); assert.equal(buckets[0].from, 0);
  assert.equal(buckets.at(-1).to, events.length); assert.equal(buckets.at(-1).kind, 'mixed');
  assert.equal(buckets.reduce((n, b) => n + b.to - b.from, 0), events.length);
  assert.deepEqual(trace.overview([]), []);
});

test('empty and malformed optional data never invents events or active content', () => {
  assert.deepEqual(trace.build(null).events, []);
  assert.deepEqual(trace.build([null, {prompt: {}, response: [], segments: [null, {steps: [null]}]}]).events, []);
  assert.equal(trace.build([{prompt: '<script>alert(1)</script>'}]).events[0].body, '<script>alert(1)</script>');
});
