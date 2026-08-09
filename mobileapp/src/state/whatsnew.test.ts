import {ReleaseNote} from '../releaseNotes';
import {capEntries, cmpVersion, countLines, linesOf, notesSince, whatsNewDue} from './whatsnew';

const note = (version: string, n: number): ReleaseNote => ({
  version,
  en: Array.from({length: n}, (_, i) => `${version} en ${i + 1}`),
  zh: Array.from({length: n}, (_, i) => `${version} zh ${i + 1}`),
});

// Newest first, the order the archive is generated in.
const history: ReleaseNote[] = [note('0.47.0', 2), note('0.46.0', 3), note('0.45.0', 2)];

describe('cmpVersion', () => {
  test('orders by numeric segment, not lexically', () => {
    // The first time a segment hits double digits, a string compare gets this backwards.
    expect(cmpVersion('0.10.0', '0.9.0')).toBeGreaterThan(0);
    expect(cmpVersion('0.45.13', '0.45.9')).toBeGreaterThan(0);
    expect(cmpVersion('1.0.0', '1.0.0')).toBe(0);
  });

  test('a missing segment is zero', () => {
    expect(cmpVersion('1.2', '1.2.0')).toBe(0);
    expect(cmpVersion('1.2', '1.2.1')).toBeLessThan(0);
  });
});

describe('notesSince', () => {
  // The point of the archive: skipping releases must not skip their notes.
  test('a multi-version jump reports every version crossed, newest first', () => {
    const got = notesSince('0.45.0', '0.47.0', history);
    expect(got.map(n => n.version)).toEqual(['0.47.0', '0.46.0']);
  });

  test('a single-version update reports just that one', () => {
    expect(notesSince('0.46.0', '0.47.0', history).map(n => n.version)).toEqual(['0.47.0']);
  });

  test('nothing new says nothing', () => {
    expect(notesSince('0.47.0', '0.47.0', history)).toEqual([]);
  });

  // There is no "new" for someone who has never seen the old.
  test('a fresh install reports nothing', () => {
    expect(notesSince(null, '0.47.0', history)).toEqual([]);
  });

  // A checkout's archive can be ahead of the binary; promising a user something their
  // build does not have is worse than saying less.
  test('versions newer than the running build are excluded', () => {
    expect(notesSince('0.45.0', '0.46.0', history).map(n => n.version)).toEqual(['0.46.0']);
  });

  test('whatsNewDue agrees with notesSince', () => {
    expect(whatsNewDue('0.45.0', '0.47.0', history)).toBe(true);
    expect(whatsNewDue('0.47.0', '0.47.0', history)).toBe(false);
    expect(whatsNewDue(null, '0.47.0', history)).toBe(false);
  });
});

describe('linesOf', () => {
  test('follows the resolved language', () => {
    expect(linesOf(note('1.0.0', 1), 'en')).toEqual(['1.0.0 en 1']);
    expect(linesOf(note('1.0.0', 1), 'zh')).toEqual(['1.0.0 zh 1']);
  });

  // Same fallback the CLI makes between a tag's `user:` and `user-zh:` blocks.
  test('falls back to the other language when its own is missing', () => {
    expect(linesOf({version: '1.0.0', en: ['only english'], zh: []}, 'zh')).toEqual(['only english']);
    expect(linesOf({version: '1.0.0', en: [], zh: ['仅中文']}, 'en')).toEqual(['仅中文']);
  });
});

describe('capEntries', () => {
  test('a typical single-version update is not truncated', () => {
    const {shown, omitted} = capEntries([note('0.47.0', 6)], 'en', 8);
    expect(shown).toHaveLength(1);
    expect(omitted).toBe(0);
  });

  // The fold appears exactly when it means something: you skipped versions.
  test('a long history folds the remainder and counts it', () => {
    const {shown, omitted} = capEntries(history, 'en', 5);
    expect(shown.map(n => n.version)).toEqual(['0.47.0', '0.46.0']); // 2 + 3 = 5
    expect(omitted).toBe(2); // 0.45.0's two bullets
    expect(countLines(shown, 'en')).toBe(5);
  });

  // "3 of 6 changes in 0.46.0" is a claim no reader can do anything with.
  test('a version is shown whole or folded, never partially', () => {
    const {shown, omitted} = capEntries(history, 'en', 4);
    expect(shown.map(n => n.version)).toEqual(['0.47.0']); // 0.46.0's 3 would exceed 4
    expect(omitted).toBe(5);
  });

  // An update that said nothing at all would be a stranger outcome than one that said a lot.
  test('the newest version always shows, even alone over the cap', () => {
    const {shown, omitted} = capEntries([note('0.47.0', 20)], 'en', 8);
    expect(shown).toHaveLength(1);
    expect(omitted).toBe(0);
  });

  test('the cap counts the language actually being read', () => {
    const mixed: ReleaseNote[] = [{version: '2.0.0', en: ['a', 'b', 'c'], zh: ['甲']}, note('1.0.0', 2)];
    expect(capEntries(mixed, 'zh', 3).shown.map(n => n.version)).toEqual(['2.0.0', '1.0.0']);
    expect(capEntries(mixed, 'en', 3).shown.map(n => n.version)).toEqual(['2.0.0']);
  });
});
