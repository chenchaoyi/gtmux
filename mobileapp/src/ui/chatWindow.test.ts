import {CHAT_WINDOW, canLoadMore, earlierLabel, nextWindow, windowedTurns} from './chatWindow';

// The chat view mounted every turn of a conversation at once. On a real long-running
// session that was 1,885 reply bubbles + 2,974 tool-step rows — the app was killed for
// memory on switching to Chat (transcript-render-bounds). These pin the window that
// replaced it, and the disclosure that keeps a partial history from reading as whole.

const many = (n: number) => Array.from({length: n}, (_, i) => `turn${i}`);

test('only the newest turns are mounted', () => {
  const {shown, hiddenHere} = windowedTurns(many(147), CHAT_WINDOW);
  expect(shown).toHaveLength(CHAT_WINDOW);
  expect(hiddenHere).toBe(147 - CHAT_WINDOW);
  // The TAIL is what you opened chat to read.
  expect(shown[shown.length - 1]).toBe('turn146');
});

test('a short conversation is shown whole, with nothing hidden', () => {
  const {shown, hiddenHere} = windowedTurns(many(5), CHAT_WINDOW);
  expect(shown).toHaveLength(5);
  expect(hiddenHere).toBe(0);
});

test('an empty history windows to nothing rather than throwing', () => {
  expect(windowedTurns([], CHAT_WINDOW)).toEqual({shown: [], hiddenHere: 0});
});

test('loading earlier grows the window a page at a time, stopping at the start', () => {
  expect(nextWindow(CHAT_WINDOW, 147)).toBe(CHAT_WINDOW * 2);
  // Never past what exists — otherwise "load earlier" would keep offering nothing.
  expect(nextWindow(CHAT_WINDOW * 7, 147)).toBe(147);
  expect(nextWindow(147, 147)).toBe(147);
});

describe('what is hidden is disclosed', () => {
  test('nothing hidden renders no control at all', () => {
    expect(earlierLabel(0, 0, false)).toBe('');
  });

  test('windowed-away turns are offered as loadable, counting BOTH causes', () => {
    // 127 held back by the window + 20 the server dropped = 147 the reader can't see.
    const label = earlierLabel(127, 20, false);
    expect(label).toContain('147');
    expect(label).toMatch(/load earlier/i);
    expect(canLoadMore(127)).toBe(true);
  });

  test('server-dropped turns say so instead of offering a control that does nothing', () => {
    const label = earlierLabel(0, 112, false);
    expect(label).toContain('112');
    expect(label).not.toMatch(/load earlier/i);
    expect(canLoadMore(0)).toBe(false);
  });

  test('bilingual', () => {
    expect(earlierLabel(30, 0, true)).toContain('载入更早');
    expect(earlierLabel(0, 30, true)).toContain('未加载');
  });
});

// A conversation that was STARTED OVER is the third reason the screen isn't the whole
// story, and the one that doesn't truncate anything: the turns ARE this conversation
// whole, and everything before it is in a session log the chat endpoint never reads.
// On 2026-08-09 a `/clear`ed HQ shift showed three bubbles on the phone and read as a
// bug — this is the line that would have answered it on the spot.
describe('a conversation that restarted says so', () => {
  // 13:24 local on 2026-08-09 — the clear that prompted this.
  const at = new Date(2026, 7, 9, 13, 24, 9).getTime() / 1000;

  test('names the command and the local clock', () => {
    const label = earlierLabel(0, 0, false, {kind: 'clear', at});
    expect(label).toContain('13:24');
    expect(label).toContain('/clear');
    expect(canLoadMore(0)).toBe(false); // nothing to load — it offers no control
  });

  test('/new reads as itself, not as /clear', () => {
    expect(earlierLabel(0, 0, false, {kind: 'new', at})).toContain('/new');
    expect(earlierLabel(0, 0, false, {kind: 'new', at})).not.toContain('/clear');
  });

  test('points at where the earlier record still is, when the surface has one', () => {
    expect(earlierLabel(0, 0, true, {kind: 'clear', at}, '动态')).toContain('动态');
    expect(earlierLabel(0, 0, true, {kind: 'clear', at})).not.toContain('动态');
    expect(earlierLabel(0, 0, false, {kind: 'clear', at}, 'Activity')).toContain('Activity');
  });

  test('an unknown clock drops the time, never the sentence', () => {
    const label = earlierLabel(0, 0, false, {kind: 'clear', at: 0});
    expect(label).toContain('/clear');
    expect(label).not.toMatch(/\d\d:\d\d/);
  });

  // Truncation outranks it: while turns are still loadable, THAT is what the reader
  // needs to know — the restart is only the end of the road once nothing is left.
  test('truncation still wins the line', () => {
    expect(earlierLabel(30, 0, false, {kind: 'clear', at})).toMatch(/load earlier/i);
    expect(earlierLabel(0, 112, false, {kind: 'clear', at})).toContain('112');
  });

  test('no reset, no line', () => {
    expect(earlierLabel(0, 0, false, undefined, 'Activity')).toBe('');
  });

  test('bilingual', () => {
    expect(earlierLabel(0, 0, true, {kind: 'clear', at})).toContain('本轮对话从 13:24 的 /clear 开始');
    expect(earlierLabel(0, 0, false, {kind: 'clear', at})).toContain('This conversation starts at 13:24');
  });
});
