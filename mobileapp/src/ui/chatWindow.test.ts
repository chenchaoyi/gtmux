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
