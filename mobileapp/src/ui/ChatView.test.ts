import {liveLabel, thinkingLabel} from './ChatView';

// A turn can spend a long time before it prints anything. The console showed your prompt
// echo and then nothing, so a thinking HQ was indistinguishable from a dead app. The
// label must therefore say BOTH that it's working and for how long — "working" alone
// can't separate a slow turn from a hung one, and only one is worth interrupting.
describe('thinkingLabel', () => {
  test('carries the elapsed time, not just a state word', () => {
    expect(thinkingLabel(1000, 1042, 'en')).toBe('Thinking… 42s');
    expect(thinkingLabel(1000, 1042, 'zh')).toBe('正在思考… 42s');
  });

  test('reads at minute and hour scale, where the reader starts wondering', () => {
    expect(thinkingLabel(1000, 1000 + 95, 'en')).toBe('Thinking… 1m35s');
    expect(thinkingLabel(1000, 1000 + 3725, 'en')).toBe('Thinking… 1h2m');
  });

  test('an unknown start time degrades to the bare state, never a fabricated duration', () => {
    for (const v of [thinkingLabel(undefined, 1042, 'en'), thinkingLabel(0, 1042, 'en')]) {
      expect(v).toBe('Thinking…');
      expect(v).not.toMatch(/\d/);
    }
  });

  test('a clock skewed behind the start time never renders a negative age', () => {
    expect(thinkingLabel(2000, 1000, 'en')).toBe('Thinking…');
  });
});

// Once the turn prints, the Live card replaces the thinking marker, and the duration has
// to come with it (%12, 2026-10-06): the card used to say only "Live".
describe('liveLabel', () => {
  test('carries the elapsed time beside the title', () => {
    expect(liveLabel(1958, 2000, 'en')).toBe('Live · 42s');
    expect(liveLabel(1958, 2000, 'zh')).toBe('正在进行 · 42s');
    expect(liveLabel(1000, 1000 + 95, 'en')).toBe('Live · 1m35s');
  });

  test('an unknown or future start is the bare title', () => {
    for (const since of [undefined, 0, 3000]) {
      expect(liveLabel(since, 2000, 'en')).toBe('Live');
    }
  });
});
