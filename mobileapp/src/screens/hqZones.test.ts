import {DigestRow, HQEvent} from '../api/client';
import {
  assessment,
  askOf,
  boardFreshness,
  decisions,
  eventPhrase,
  eventSession,
  fleetCounts,
  hasNewActivity,
  initialZone,
  sessionName,
  windowNo,
  workerRows,
} from './hqZones';

// The HQ page's logic (hq-command-page). These test the REAL module the screen imports —
// the suite this replaces mirrored the old fleet board's grouping inside the test file,
// so it would have kept passing no matter what the screen did.

const mk = (o: Partial<DigestRow>): DigestRow => ({agent: 'Claude Code', source: 'tmux', status: 'idle', ...o});
const ev = (o: Partial<HQEvent>): HQEvent => ({ts: 1000, event: 'Stop', ...o});

const FLEET = [
  mk({loc: 'HQ:0.0', status: 'working', role: 'supervisor'}),
  mk({loc: 'api:0.0', status: 'waiting', ask: 'run the test suite?', since: 100}),
  mk({loc: 'web:1.0', status: 'waiting', ask: 'merge this?', since: 50}),
  mk({loc: 'docs:0.0', status: 'idle'}),
];

test('the supervisor never appears inside its own page', () => {
  expect(workerRows(FLEET).some(r => r.role === 'supervisor')).toBe(false);
  expect(fleetCounts(FLEET)).toEqual({waiting: 2, working: 0, idle: 1});
});

test('decisions are the blocked sessions, longest-stuck first', () => {
  // web has been waiting since an EARLIER timestamp, so it has been stuck longer.
  expect(decisions(FLEET).map(r => r.loc)).toEqual(['web:1.0', 'api:0.0']);
});

describe('assessment names who needs you', () => {
  test('one waiting, others normal', () => {
    const one = [FLEET[0], FLEET[1], FLEET[3]];
    expect(assessment(one, false)).toBe('api needs you · 1 others normal');
    expect(assessment(one, true)).toBe('api 在等你拍板 · 其余 1 个正常');
  });

  test('several waiting collapses to a count', () => {
    expect(assessment(FLEET, false)).toBe('2 sessions need you');
  });

  test('a quiet fleet says so — never an empty line', () => {
    expect(assessment([FLEET[0], FLEET[3]], true)).toBe('都正常 · 无需你介入');
  });

  test('no workers at all is still a sentence', () => {
    expect(assessment([FLEET[0]], false)).toBe('no other agent sessions');
  });
});

test('the ask is the card body; a row without one still says something', () => {
  expect(askOf(FLEET[1], false)).toBe('run the test suite?');
  expect(askOf(mk({goal: 'refactor auth'}), false)).toBe('refactor auth');
  expect(askOf(mk({}), true)).toContain('打开会话');
  expect(askOf(mk({}), false)).not.toBe('');
});

test('session name and window number come off the locator', () => {
  expect(sessionName(FLEET[2])).toBe('web');
  expect(windowNo(FLEET[2])).toBe('1');
  expect(windowNo(mk({loc: 'bare'}))).toBe('');
});

test('opening HQ lands on the block when there is one', () => {
  expect(initialZone(FLEET)).toBe('calls');
  expect(initialZone([FLEET[0], FLEET[3]])).toBe('console');
});

describe('event feed reads as prose, not a log line', () => {
  test('a waiting kind says what kind', () => {
    expect(eventPhrase(ev({event: 'Waiting', kind: 'permission'}), false)).toBe('wants permission');
    expect(eventPhrase(ev({event: 'Waiting', kind: 'question'}), true)).toBe('有问题问你');
  });

  test('a turn that ended on a question is not "finished"', () => {
    expect(eventPhrase(ev({event: 'Stop', class: 'asking'}), false)).toBe('ended on a question');
    expect(eventPhrase(ev({event: 'Stop'}), false)).toBe('finished a turn');
  });

  test('a crashed turn never reads as a finish', () => {
    expect(eventPhrase(ev({event: 'StopFailure'}), false)).toBe('a turn crashed');
  });

  test('an instruction is distinguished from harness input', () => {
    expect(eventPhrase(ev({event: 'UserPromptSubmit', origin: 'instruction'}), false)).toBe('got an instruction');
    expect(eventPhrase(ev({event: 'UserPromptSubmit'}), false)).toBe('got input');
  });

  test('an unknown event degrades to its name rather than blank', () => {
    expect(eventPhrase(ev({event: 'SomethingNew'}), true)).toBe('SomethingNew');
  });

  test('the session handle falls back through loc/pane', () => {
    expect(eventSession(ev({session: 'api'}))).toBe('api');
    expect(eventSession(ev({loc: 'web:0.0'}))).toBe('web');
    expect(eventSession(ev({pane: '%7'}))).toBe('%7');
  });
});

test('the activity tab flags unread by sequence, and an empty feed is never "new"', () => {
  const feed = [ev({seq: 108}), ev({seq: 107})];
  expect(hasNewActivity(feed, 0)).toBe(true);
  expect(hasNewActivity(feed, 108)).toBe(false);
  expect(hasNewActivity([], 0)).toBe(false);
  // A legacy ledger without seq falls back to the timestamp.
  expect(hasNewActivity([ev({ts: 500})], 400)).toBe(true);
});

test('board freshness is shown, because an old assessment is worth less', () => {
  const now = 10_000;
  expect(boardFreshness(now - 7200, now, false)).toBe('situation board · 2h ago');
  expect(boardFreshness(now - 120, now, true)).toBe('态势板 · 2m前');
  // No timestamp: label it plainly rather than claiming a freshness we don't have.
  expect(boardFreshness(undefined, now, false)).toBe('situation board');
});
