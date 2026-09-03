import {clockKnown, clockSkewSec, noteServerDate, resetClock, serverNowSec} from './clock';
import {thinkingLabel} from '../ui/ChatView';

// The duration beside a working session is computed against a timestamp the MAC produced.
// Subtracting it from the phone's own clock is only right while the two agree, and a phone
// that runs fast turns a twelve-second turn into minutes — which is exactly the number the
// label exists to make trustworthy ("working" alone cannot tell a thinking turn from a
// hung one, and only one of those is worth interrupting).
beforeEach(() => resetClock());

const dateHeaderFor = (epochMs: number) => new Date(epochMs).toUTCString();

test('with no response seen yet, the server clock IS the local clock', () => {
  expect(clockKnown()).toBe(false);
  expect(Math.abs(serverNowSec() - Math.floor(Date.now() / 1000))).toBeLessThanOrEqual(1);
});

test('a phone running fast no longer inflates the duration', () => {
  // The Mac says it is three minutes earlier than the phone thinks.
  const serverMs = Date.now() - 180_000;
  noteServerDate(dateHeaderFor(serverMs));
  expect(clockSkewSec()).toBeLessThanOrEqual(-179);

  // A turn that started twelve seconds ago, in the server's frame.
  //
  // Asserted as a RANGE, not a string: the Date header has one-second resolution and the
  // reading includes the response's own flight time, so the estimate is good to about a
  // second. That is the accuracy this is documented to have, and pinning an exact string
  // would make the test fail on the timing rather than on the behaviour.
  const since = Math.floor(serverMs / 1000) - 12;
  const seconds = (label: string): number => {
    const m = /(?:(\d+)m)?(\d+)s/.exec(label);
    return m ? (m[1] ? Number(m[1]) * 60 : 0) + Number(m[2]) : -1;
  };
  const corrected = seconds(thinkingLabel(since, serverNowSec(), 'en'));
  expect(corrected).toBeGreaterThanOrEqual(11);
  expect(corrected).toBeLessThanOrEqual(13);
  // What it used to say, with the phone's own clock: minutes for a twelve-second turn.
  expect(seconds(thinkingLabel(since, Math.floor(Date.now() / 1000), 'en'))).toBeGreaterThan(180);
});

test('a phone running slow still falls back rather than counting backwards', () => {
  // The guard for this direction already existed; it must survive the new clock.
  const serverMs = Date.now() + 120_000;
  noteServerDate(dateHeaderFor(serverMs));
  const since = Math.floor(serverMs / 1000) + 30; // a start still ahead of "now"
  expect(thinkingLabel(since, serverNowSec(), 'en')).toBe('Thinking…');
});

test('an unreadable or absent header leaves the estimate alone', () => {
  noteServerDate(dateHeaderFor(Date.now() - 60_000));
  const before = clockSkewSec();
  noteServerDate(null);
  noteServerDate('not a date');
  expect(clockSkewSec()).toBe(before);
});

test('the latest reading wins, so a corrected phone clock is followed at once', () => {
  // A smoothed estimate would drag across an NTP step for the length of its window.
  noteServerDate(dateHeaderFor(Date.now() - 300_000));
  expect(clockSkewSec()).toBeLessThan(-290);
  noteServerDate(dateHeaderFor(Date.now()));
  expect(Math.abs(clockSkewSec())).toBeLessThanOrEqual(1);
});
