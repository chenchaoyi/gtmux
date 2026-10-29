/// <reference types="node" />
import { agentName, build, filter, overview } from './model';

test.each(['codex', 'claude', 'kimi', 'opencode', 'dsh'])(
  '%s indexes recorded inputs without fabricating outcomes',
  agent => {
    const result = build(
      [
        {
          session_id: 'one',
          agent,
          prompt: 'Review',
          time: '2026-10-28T10:00:00Z',
          segments: [
            {
              text: 'Reading',
              steps: [
                {
                  title: 'exec',
                  input: 'line one\n  line two',
                  workdir: '/project',
                  details_truncated: true,
                },
              ],
            },
            { text: 'Done' },
          ],
        },
      ],
      { home_id: 'home' },
    );
    expect(result.events.map(e => e.kind)).toEqual([
      'user',
      'assistant',
      'tool',
      'assistant',
    ]);
    expect(result.events[2]).toMatchObject({
      body: 'line one\n  line two',
      sourceIndex: 0,
      step: { workdir: '/project', truncated: true },
    });
    expect(result.events[2]).not.toHaveProperty('duration');
    expect(result.events[2]).not.toHaveProperty('result');
  },
);
test('identities survive history prepend and distinguish homes and identical clockless prompts', () => {
  const turn = {
    session_id: 'one',
    prompt: 'Same',
    time: '2026-10-28T10:00:00Z',
    response: 'First',
  };
  const initial = build([turn], { home_id: 'a' }).events;
  const after = build(
    [
      { session_id: 'older', prompt: 'Older' },
      { ...turn, response: 'Streamed' },
    ],
    { home_id: 'a' },
  ).events;
  expect(after[1].key).toBe(initial[0].key);
  expect(after[2].key).toBe(initial[1].key);
  expect(after[1].sourceIndex).toBe(1);
  expect(build([turn], { home_id: 'b' }).events[0].key).not.toBe(
    initial[0].key,
  );
  const twins = build([{ prompt: 'Same' }, { prompt: 'Same' }]).events;
  expect(new Set(twins.map(e => e.key)).size).toBe(2);
});
test('search and overview cover all 5002 records rather than a rendered window', () => {
  const index = build(
    Array.from({ length: 2501 }, (_, i) => ({
      session_id: 'long',
      prompt: 'Request ' + i,
      response: 'Result ' + i,
    })),
  );
  expect(index.events).toHaveLength(5002);
  expect(filter(index.events, 'Result 2500', 'assistant')).toHaveLength(1);
  const map = overview(index.events);
  expect(map).toHaveLength(120);
  expect(map[0].from).toBe(0);
  expect(map.at(-1)?.to).toBe(5002);
});
test('malformed optional data and provider-like object keys remain literal', () => {
  expect(build(null, null).events).toEqual([]);
  expect(
    build([null, { prompt: {}, segments: [null, { steps: [null] }] }]).events,
  ).toEqual([]);
  expect(agentName('__proto__')).toBe('__proto__');
  expect(agentName('constructor')).toBe('constructor');
});

// Both consumers run these same public-transcript fixtures. They are synthetic.
const fixtures: {
  name: string;
  turns: unknown;
  context: import('./model').TraceContext;
  kinds: string[];
  bodies: string[];
  sessions: string[];
}[] = require('./fixtures.json');
const generated = require('../../../internal/server/web/trace-model.js');
test.each(fixtures)(
  '$name agrees with the generated browser consumer',
  fixture => {
    const index = build(fixture.turns, fixture.context);
    expect(index.events.map(event => event.kind)).toEqual(fixture.kinds);
    expect(index.events.map(event => event.body)).toEqual(fixture.bodies);
    expect(index.sessions.map(session => session.session)).toEqual(
      fixture.sessions,
    );
    expect(generated.build(fixture.turns, fixture.context)).toEqual(index);
  },
);
test('the checked-in browser asset matches its typed source and generator', () => {
  const { execFileSync } = require('node:child_process');
  const path = require('node:path');
  expect(() =>
    execFileSync(process.execPath, [
      path.resolve(__dirname, '../../scripts/gen-trace-web.cjs'),
      '--check',
    ]),
  ).not.toThrow();
});
