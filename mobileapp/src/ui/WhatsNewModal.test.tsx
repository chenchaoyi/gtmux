import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ReleaseNote} from '../releaseNotes';
import {Lang} from '../i18n';
import {WhatsNewModal} from './WhatsNewModal';
import {paletteFor} from './theme';

const pal = paletteFor('light');

const note = (version: string, n: number): ReleaseNote => ({
  version,
  en: Array.from({length: n}, (_, i) => `${version} en ${i + 1}`),
  zh: Array.from({length: n}, (_, i) => `${version} zh ${i + 1}`),
});

function render(entries: ReleaseNote[], lang: Lang = 'en', showAll = false) {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <WhatsNewModal visible entries={entries} pal={pal} lang={lang} showAll={showAll} onClose={() => {}} />,
    );
  });
  return tree!;
}

const texts = (tree: renderer.ReactTestRenderer): string[] =>
  tree.root.findAllByType('Text' as never).flatMap(n => {
    const c = n.props.children;
    return typeof c === 'string' ? [c] : [];
  });

describe('WhatsNewModal', () => {
  // A heading over the only group is noise; a jump across versions needs them to tell the
  // reader WHICH release each change came from. The header chip always names the newest,
  // so the signal is whether the version appears a SECOND time as a group heading.
  const occurrences = (tree: renderer.ReactTestRenderer, s: string) =>
    texts(tree).filter(t => t === s).length;

  test('one version is named once (the header chip), with no group heading', () => {
    expect(occurrences(render([note('0.47.0', 2)]), '0.47.0')).toBe(1);
  });

  // …and when the headings appear, the chip steps aside: printing the newest version
  // twice, once in the chip and again as the first heading, is not structure but noise
  // (seen on device, 2026-08-09).
  test('several versions each get a heading, and none is named twice', () => {
    const tree = render([note('0.47.0', 1), note('0.46.0', 1)]);
    expect(occurrences(tree, '0.47.0')).toBe(1);
    expect(occurrences(tree, '0.46.0')).toBe(1);
  });

  // The fold is the "you skipped versions" affordance — it must name how many are behind it.
  test('a capped summary names the remainder and expands in place', () => {
    const entries = [note('0.47.0', 6), note('0.46.0', 5), note('0.45.0', 4)];
    const tree = render(entries);
    expect(texts(tree).some(t => t.includes('+9 more'))).toBe(true);
    // The folded versions are not rendered until asked for.
    expect(texts(tree)).not.toContain('0.46.0 en 1');

    act(() => {
      tree.root.findAll(n => n.props.accessibilityRole === 'button')[0].props.onPress();
    });
    expect(texts(tree)).toContain('0.46.0 en 1');
    expect(texts(tree).some(t => t.includes('more'))).toBe(false);
  });

  test('showAll opens expanded (the Settings entry)', () => {
    const tree = render([note('0.47.0', 6), note('0.46.0', 5)], 'en', true);
    expect(texts(tree)).toContain('0.46.0 en 1');
    expect(texts(tree).some(t => t.includes('more'))).toBe(false);
  });

  test('renders the reader language, and its buttons', () => {
    expect(texts(render([note('1.0.0', 1)], 'zh'))).toEqual(expect.arrayContaining(['1.0.0 zh 1', '更新内容', '知道了']));
    expect(texts(render([note('1.0.0', 1)], 'en'))).toEqual(expect.arrayContaining(['1.0.0 en 1', "What's new", 'Got it']));
  });
});

// The scroll bug, pinned structurally. The card used to be wrapped in the backdrop
// Pressable (and itself be a Pressable, to stop taps falling through), so a touch was
// claimed on START by the JS responder and the ScrollView never got the gesture: the card
// scrolled in fits or not at all. The rule that fixes it is a rule about the TREE, so the
// test is about the tree — no Pressable may be an ancestor of the ScrollView.
describe('WhatsNewModal scrolling', () => {
  test('no Pressable wraps the ScrollView', () => {
    const tree = render([note('0.47.0', 20)]);
    const scroll = tree.root.findByType(
      require('react-native').ScrollView as never,
    ) as {parent: {type: unknown; parent: unknown} | null};
    for (let n = scroll.parent; n; n = n.parent as never) {
      const name = typeof n.type === 'function' ? (n.type as {name?: string}).name : String(n.type);
      expect(name).not.toBe('Pressable');
    }
  });

  // …and the backdrop still dismisses, which is what the wrapping was for.
  test('tapping the backdrop closes', () => {
    let closed = false;
    let tree: renderer.ReactTestRenderer;
    act(() => {
      tree = renderer.create(
        <WhatsNewModal visible entries={[note('1.0.0', 1)]} pal={pal} lang="en" onClose={() => {
          closed = true;
        }} />,
      );
    });
    const backdrop = tree!.root.findAll(n => n.props.accessibilityLabel === 'Close')[0];
    act(() => backdrop.props.onPress());
    expect(closed).toBe(true);
  });
});
