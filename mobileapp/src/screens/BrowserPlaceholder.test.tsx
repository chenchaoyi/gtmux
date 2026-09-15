import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {BrowserPlaceholder} from './PaneBrowserScreen';
import {TestIds} from '../constants/testIds';
import {paletteFor} from '../ui/theme';

// Opening "All panes" showed a blank page until the first read of /api/panes came back
// (2026-09-15: 「点击 all panes 后需要增加 loading screen」), and a blank page is
// indistinguishable from "no panes". These pin the three states apart: still reading,
// read and empty, read and nothing matched.
function render(props: Partial<React.ComponentProps<typeof BrowserPlaceholder>> = {}) {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <BrowserPlaceholder loaded={false} q="" isGuest={false} macName="studio" zh={false} pal={paletteFor('dark')} {...props} />,
    );
  });
  return tree!;
}
const strings = (t: renderer.ReactTestRenderer): string =>
  t.root
    .findAllByType(Text)
    .flatMap(n => ([] as unknown[]).concat(n.props.children as unknown[]).filter(c => typeof c === 'string'))
    .join(' ');

test('before the first read lands: the loading mark and what it is reading, never "no panes"', () => {
  const t = render();
  expect(t.root.findAllByProps({testID: TestIds.panes.loading}).length).toBeGreaterThan(0);
  expect(t.root.findAllByProps({testID: 'loading-mark'}).length).toBeGreaterThan(0);
  expect(strings(t)).toContain('Reading panes on studio');
  expect(strings(t)).not.toContain('No tmux panes');
});

test('the caption reads in the reader language', () => {
  expect(strings(render({zh: true}))).toContain('正在读取 studio 的 pane');
});

test('once read, an empty result is stated and the mark is gone', () => {
  const t = render({loaded: true});
  expect(t.root.findAllByProps({testID: 'loading-mark'})).toHaveLength(0);
  expect(strings(t)).toContain('No tmux panes');
  expect(strings(t)).toContain('studio');
});

test('a search with no match says so, without the hint about opening a window', () => {
  const s = strings(render({loaded: true, q: 'zzz'}));
  expect(s).toContain('No panes match');
  expect(s).not.toContain('Open a tmux window');
});
