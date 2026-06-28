import {mergeVoiceText} from './useVoiceInput';

describe('mergeVoiceText', () => {
  it('uses the transcript alone when nothing was typed', () => {
    expect(mergeVoiceText('', 'hello there')).toBe('hello there');
  });

  it('appends the transcript after typed text with one space', () => {
    expect(mergeVoiceText('run', 'the tests')).toBe('run the tests');
  });

  it('does not double-space a base that already ends in a space', () => {
    // a partial replaces the live region each time, so the base is stable —
    // we only ever add one separator.
    expect(mergeVoiceText('run', 'the')).toBe('run the');
    expect(mergeVoiceText('run', 'the tests now')).toBe('run the tests now');
  });
});
