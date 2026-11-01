import { build, mergeHistory, prependHistory } from './model';
import { TranscriptTurn } from '../api/client';
const turn = (n: number, response = 'reply'): TranscriptTurn => ({
  session_id: 'session',
  record_id: `source:${n}`,
  turn_ordinal: n,
  prompt: `question ${n}`,
  response,
});
test('source ordinal is shared by all events and remains stable after prepend', () => {
  const latest = [turn(158), turn(159)];
  const a = build(latest);
  const b = build([turn(157), ...latest]);
  expect(a.events.map(e => e.turn)).toEqual([158, 158, 159, 159]);
  expect(b.events.slice(2).map(e => e.key)).toEqual(a.events.map(e => e.key));
});
test('history keeps live streaming updates and does not join an unseen gap', () => {
  const held = [turn(156), turn(157), turn(158)];
  expect(mergeHistory(held, [turn(158, 'updated'), turn(159)])).toEqual({
    turns: [turn(156), turn(157), turn(158, 'updated'), turn(159)],
    gap: false,
  });
  expect(mergeHistory(held, [turn(159)])).toEqual({
    turns: [...held, turn(159)],
    gap: false,
  });
  expect(mergeHistory(held, [turn(170)])).toEqual({ turns: held, gap: true });
  expect(
    mergeHistory(held, [{ ...turn(158), record_id: 'replacement' }]).gap,
  ).toBe(true);
});
test('an earlier page must be adjacent and from the same session', () => {
  expect(prependHistory([turn(156), turn(157)], [turn(158)])).toEqual([
    turn(156),
    turn(157),
    turn(158),
  ]);
  expect(() => prependHistory([turn(156)], [turn(158)])).toThrow(
    'History anchor changed',
  );
  expect(() =>
    prependHistory([{ ...turn(157), session_id: 'other' }], [turn(158)]),
  ).toThrow();
});
