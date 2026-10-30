import { build, groupTurns } from './model';
import { timelineRows, tracePreview } from './timeline';
const calls = Array.from({ length: 8 }, (_, i) => ({
  title: 'exec',
  input: `line ${i}\n  keep indent`,
  operation: 'command',
}));
const turn = {
  session_id: 'current',
  time: '2026-10-28T10:00:00Z',
  prompt: 'Inspect',
  segments: [{ text: 'Reading', steps: calls }, { text: 'Checking tests' }],
};

test('turn identity and expansion survive prepend while calls remain individually reachable', () => {
  const original = groupTurns(build([turn], { home_id: 'one' }).events)[0];
  const groups = groupTurns(
    build([{ ...turn, prompt: 'Earlier' }, turn], { home_id: 'one' }).events,
  );
  expect(groups[1].key).toBe(original.key);
  expect(groups[1].calls).toBe(8);
  const rows = timelineRows(groups, { [original.key]: true }, {});
  expect(rows.filter(row => row.type === 'calls')).toHaveLength(1);
  expect(rows.filter(row => row.type === 'event')).toHaveLength(3);
  const batch = rows.find(row => row.type === 'calls')!;
  const expanded = timelineRows(
    groups,
    { [original.key]: true },
    { [batch.key]: true },
  );
  expect(
    expanded
      .filter(row => row.type === 'event')
      .map(row => row.type === 'event' && row.event.body),
  ).toEqual(original.events.map(event => event.body));
  expect(groupTurns(build([turn], { home_id: 'two' }).events)[0].key).not.toBe(
    original.key,
  );
});

test('search reveals matching individual calls regardless of collapsed history', () => {
  const groups = groupTurns(build([turn]).events);
  const rows = timelineRows(groups, {}, {}, true);
  expect(rows.filter(row => row.type === 'event')).toHaveLength(
    groups[0].events.length,
  );
  expect(rows.some(row => row.type === 'calls')).toBe(false);
});

test('only recognized image envelopes are removed from a preview; source remains literal', () => {
  const raw =
    '<image name=[Image #1] path="/private/image.png"></image>\nPlease inspect';
  expect(tracePreview(raw, 'en')).toBe('Please inspect');
  expect(
    tracePreview(
      '<image name=[Image #1] path="/private/image.png"></image>',
      'zh',
    ),
  ).toBe('图片消息');
  expect(tracePreview('Explain the <image> tag', 'en')).toBe(
    'Explain the <image> tag',
  );
  expect(raw).toContain('/private/image.png');
});

test('bulk call folding includes single/two-call runs and stays independent of turns', () => {
  const groups = groupTurns(
    build([
      {
        prompt: 'Request',
        segments: [
          { steps: calls.slice(0, 2) },
          { text: 'Update', steps: calls.slice(2, 3) },
        ],
      },
    ]).events,
  );
  const closed = timelineRows(groups, {}, {}, false, {
    turns: true,
    calls: false,
  });
  const batches = closed.filter(row => row.type === 'calls');
  expect(batches.map(row => row.type === 'calls' && row.events.length)).toEqual(
    [2, 1],
  );
  expect(
    closed.some(row => row.type === 'event' && row.event.kind === 'tool'),
  ).toBe(false);
  expect(
    timelineRows(groups, {}, {}, false, { turns: false, calls: true }).map(
      row => row.type,
    ),
  ).toEqual(['turn']);
  const individual = timelineRows(
    groups,
    {},
    { [batches[0].key]: true },
    false,
    { turns: true, calls: false },
  );
  expect(
    individual.filter(row => row.type === 'event' && row.event.kind === 'tool'),
  ).toHaveLength(2);
  const searching = timelineRows(groups, {}, {}, true, {
    turns: false,
    calls: false,
  });
  expect(searching.filter(row => row.type === 'event')).toHaveLength(
    groups[0].events.length,
  );
  expect(searching.some(row => row.type === 'calls')).toBe(false);
});
