import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {SnippetsPicker} from './SnippetsPicker';
import {SnippetsModal} from './SnippetsModal';
import {paletteFor} from './theme';

const pal = paletteFor('dark');

function texts(el: React.ReactElement): string[] {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(el);
  });
  return tree!.root.findAllByType(Text).map(t => {
    const c = t.props.children;
    return Array.isArray(c) ? c.join('') : String(c ?? '');
  });
}

// Pins the 2026-08 user rename: every user-visible "Snippets" label reads
// "Quick replies" (EN) / 「常用语」 (ZH). Internal names/testIDs keep `snippets`.
test('SnippetsPicker sheet titles: Quick replies / 常用语', () => {
  const base = {visible: true, snippets: ['x'], pal, onPick: () => {}, onManage: () => {}, onClose: () => {}};
  expect(texts(<SnippetsPicker {...base} lang="en" />)).toContain('Quick replies');
  expect(texts(<SnippetsPicker {...base} lang="zh" />)).toContain('常用语');
});

test('SnippetsModal manage titles: Quick replies / 常用语', () => {
  const base = {visible: true, snippets: [], pal, onChange: () => {}, onClose: () => {}};
  expect(texts(<SnippetsModal {...base} lang="en" />)).toContain('Quick replies');
  expect(texts(<SnippetsModal {...base} lang="zh" />)).toContain('常用语');
});

// The same tap-catcher a11y rule as PickerSheet: the sheet/backdrop touchables
// opt OUT of accessibility so the sheet's children stay individual AX elements.
test('SnippetsPicker tap-catchers are accessible={false}', () => {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <SnippetsPicker visible snippets={['x']} pal={pal} lang="en" onPick={() => {}} onManage={() => {}} onClose={() => {}} />,
    );
  });
  const catchers = tree!.root.findAll(n => n.props.activeOpacity === 1 && n.props.accessible !== undefined);
  expect(catchers.length).toBeGreaterThan(0);
  for (const c of catchers) expect(c.props.accessible).toBe(false);
});
