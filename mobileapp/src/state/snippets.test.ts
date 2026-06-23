import {addSnippet, removeSnippet} from './snippets';

describe('addSnippet', () => {
  it('appends a new snippet', () => {
    expect(addSnippet(['a'], 'b')).toEqual(['a', 'b']);
  });
  it('trims whitespace', () => {
    expect(addSnippet([], '  hi  ')).toEqual(['hi']);
  });
  it('ignores empty / whitespace-only input', () => {
    expect(addSnippet(['a'], '')).toEqual(['a']);
    expect(addSnippet(['a'], '   ')).toEqual(['a']);
  });
  it('de-duplicates (exact, after trim)', () => {
    expect(addSnippet(['a', 'b'], 'a')).toEqual(['a', 'b']);
    expect(addSnippet(['a'], ' a ')).toEqual(['a']);
  });
  it('does not mutate the input array', () => {
    const orig = ['a'];
    addSnippet(orig, 'b');
    expect(orig).toEqual(['a']);
  });
});

describe('removeSnippet', () => {
  it('removes every exact match', () => {
    expect(removeSnippet(['a', 'b', 'a'], 'a')).toEqual(['b']);
  });
  it('is a no-op when absent', () => {
    expect(removeSnippet(['a', 'b'], 'z')).toEqual(['a', 'b']);
  });
  it('does not mutate the input array', () => {
    const orig = ['a', 'b'];
    removeSnippet(orig, 'a');
    expect(orig).toEqual(['a', 'b']);
  });
});
