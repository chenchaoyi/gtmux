import {Entry} from './index';
import {countProblems, describeEntry, describeRecord, diagLines, groupByDay} from './lines';

const at = (ts: string, e: Partial<Entry>): Entry =>
  ({ts, level: 'info', component: 'phone', kind: 'diag', event: 'x', ...e} as Entry);

describe('describeEntry', () => {
  it('describes an incomplete request without inferring the Mac is offline', () => {
    const {title, detail} = describeEntry(
      at('2026-09-20T13:58:00.000+08:00', {
        level: 'warn', event: 'api.failed',
        attrs: {route: 'GET /api/agents', ms: 6000, repeats: 4},
      }),
      false,
    );
    expect(title).toBe('Request did not complete');
    expect(detail).toContain('GET /api/agents · no response (6s)');
    expect(detail).toContain('then 4 more times in a minute');
  });

  it('does not infer a timeout from a short network failure', () => {
    // A generic network error and elapsed time do not identify the transport cause.
    const {detail} = describeEntry(
      at('2026-09-20T13:58:00.000+08:00', {
        level: 'warn', event: 'api.failed',
        attrs: {route: 'GET /api/agents', ms: 18, error: 'Network request failed'},
      }),
      false,
    );
    expect(detail).toBe('GET /api/agents · no response (18ms)');
  });

  it('keeps the library own English out of the sentence', () => {
    const {detail} = describeEntry(
      at('2026-09-20T13:57:00.000+08:00', {
        level: 'warn', event: 'sse.disconnected',
        attrs: {error: 'Could not connect to the server.'},
      }),
      true,
    );
    expect(detail).toBeUndefined();
  });

  it('keeps an HTTP rejection distinct from a transport failure', () => {
    const refused = describeEntry(
      at('2026-09-20T13:58:00.000+08:00', {level: 'warn', event: 'api.failed', attrs: {route: 'GET /api/agents', ms: 210, status: 401}}),
      false,
    );
    expect(refused.title).toBe('Server request failed');
    expect(refused.detail).toContain('HTTP 401');
  });

  it('names why a pairing was refused, in the words the pairing screen uses', () => {
    const {title, detail} = describeEntry(
      at('2026-09-20T11:19:00.000+08:00', {
        level: 'warn', kind: 'act', event: 'act.pair', target: 'tunnel.example',
        outcome: 'refused', attrs: {reason: 'codeInvalid', status: 401},
      }),
      false,
    );
    expect(title).toBe('A pairing did not go through');
    expect(detail).toContain('tunnel.example');
    expect(detail).toContain('scan a fresh one');
  });

  it('says how long the live connection was gone', () => {
    const {title, detail} = describeEntry(
      at('2026-09-20T14:03:00.000+08:00', {event: 'sse.connected', attrs: {downSec: 300}}),
      false,
    );
    expect(title).toBe('The live connection is back');
    expect(detail).toBe('it was down for 5 minutes');
  });

  it('falls back to the entry message for an event it has not been taught', () => {
    const {title, detail} = describeEntry(
      at('2026-09-20T09:00:00.000+08:00', {event: 'phone.something', msg: 'something happened', attrs: {n: 3}}),
      false,
    );
    expect(title).toBe('something happened');
    expect(detail).toBe('n=3');
  });

  it('answers in Chinese when asked', () => {
    const {title} = describeEntry(at('2026-09-20T14:06:00.000+08:00', {event: 'phone.start', attrs: {version: '1.0.35'}}), true);
    expect(title).toBe('app 启动');
  });
});

describe('the record as a list', () => {
  const entries = [
    at('2026-09-19T22:04:00.000+08:00', {kind: 'act', event: 'act.push.register', outcome: 'ok', attrs: {kinds: 'waiting,done'}}),
    at('2026-09-20T13:57:00.000+08:00', {level: 'warn', event: 'sse.disconnected'}),
    at('2026-09-20T14:06:00.000+08:00', {event: 'phone.start', attrs: {version: '1.0.35'}}),
  ];

  it('puts the newest first and keeps the clock', () => {
    const lines = diagLines(entries, false);
    expect(lines.map(l => l.clock)).toEqual(['14:06', '13:57', '22:04']);
    expect(lines[1].problem).toBe(true);
    expect(lines[0].problem).toBe(false);
  });

  it('groups by day and names today and yesterday', () => {
    const now = new Date('2026-09-20T15:00:00+08:00');
    const sections = groupByDay(diagLines(entries, false), now, false);
    expect(sections.map(s => s.title)).toEqual(['Today', 'Yesterday']);
    expect(sections[0].lines).toHaveLength(2);
  });

  it('counts warnings and errors as the problems', () => {
    expect(countProblems(entries)).toBe(1);
  });
});

describe('describeRecord', () => {
  it('leads with problems when there are any', () => {
    expect(describeRecord({count: 312, bytes: 48_000}, 2, false)).toBe('2 problems');
    expect(describeRecord({count: 312, bytes: 48_000}, 1, false)).toBe('1 problem');
  });

  it('otherwise says how much is kept', () => {
    expect(describeRecord({count: 312, bytes: 48_000}, 0, false)).toBe('312 entries');
    expect(describeRecord({count: 0, bytes: 0}, 0, false)).toBe('Nothing yet');
  });
});

// A count of one reads in the singular. "then 1 more times in a minute" was on the
// simulator's record (2026-10-05).
describe('a count of one', () => {
  it('says "time", "entry" and "second"', () => {
    const {detail} = describeEntry(
      at('2026-10-05T07:03:00.000+08:00', {level: 'warn', event: 'api.failed', attrs: {route: 'GET /api/awake', ms: 18, repeats: 1}}),
      false,
    );
    expect(detail).toBe('GET /api/awake · no response (18ms) · then 1 more time in a minute');
    expect(describeRecord({count: 1, bytes: 200}, 0, false)).toBe('1 entry');
    const back = describeEntry(at('2026-10-05T07:03:00.000+08:00', {event: 'sse.connected', attrs: {downSec: 1}}), false);
    expect(back.detail).toBe('it was down for 1 second');
  });
});

test('legacy 304 warnings are unchanged content, not problems, without rewriting the source', () => {
  const entry = at('2026-10-15T20:55:00+08:00', {level: 'warn', event: 'api.failed', attrs: {route: 'GET /p35047/api/transcript', status: 304}});
  const raw = JSON.stringify(entry);
  expect(describeEntry(entry, false).title).toBe('Content unchanged');
  expect(describeEntry(entry, true).title).toBe('内容未变化');
  expect(diagLines([entry], false)[0].problem).toBe(false);
  expect(countProblems([entry])).toBe(0);
  expect(JSON.stringify(entry)).toBe(raw);
  expect(countProblems([{...entry, attrs: {...entry.attrs, status: 401}}])).toBe(1);
  expect(countProblems([{...entry, attrs: {...entry.attrs, route: 'POST /api/send'}}])).toBe(1);
});

test('only an explicit deadline is described as a timeout in both languages', () => {
  const timeout = at('2026-10-15T20:55:00+08:00', {level: 'warn', event: 'api.failed', attrs: {route: 'POST /api/push/unregister', reason: 'timeout', ms: 4000}});
  expect(describeEntry(timeout, false)).toEqual({title: 'Request timed out', detail: 'POST /api/push/unregister · timed out after 4s'});
  expect(describeEntry(timeout, true).title).toBe('请求超时');
  expect(describeEntry({...timeout, attrs: {...timeout.attrs, reason: 'network'}}, false).title).toBe('Request did not complete');
});
