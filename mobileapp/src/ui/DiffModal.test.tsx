import React from 'react';
import renderer, { act } from 'react-test-renderer';
import { FlatList, ScrollView, Text, TouchableOpacity } from 'react-native';
import { DiffModal } from './DiffModal';
import { paletteFor } from './theme';
import { diffRows } from './diff';
const props = {
  visible: true,
  paneId: '%synthetic',
  pal: paletteFor('dark'),
  lang: 'en' as const,
  onClose: jest.fn(),
};
let tree: renderer.ReactTestRenderer;
afterEach(() => {
  if (tree) act(() => tree.unmount());
});
test('large diffs mount only a window of native rows and retain the final line', async () => {
  const lines = Array.from({ length: 40000 }, (_, i) => '+line ' + i);
  await act(async () => {
    tree = renderer.create(
      <DiffModal
        {...props}
        client={{ diff: async () => lines.join('\n') } as any}
      />,
    );
  });
  const list = tree.root.findByType(FlatList);
  expect(list.props.data).toHaveLength(40000);
  expect(list.props.data.at(-1)).toBe('+line 39999');
  expect(tree.root.findAllByType(Text).length).toBeLessThan(200);
  expect(list.props.getItemLayout(undefined, 39999).offset).toBeGreaterThan(
    600000,
  );
});
test('a failed read is an error with retry, not an empty repository', async () => {
  const diff = jest
    .fn()
    .mockRejectedValueOnce(new Error('offline'))
    .mockResolvedValueOnce('+recovered');
  await act(async () => {
    tree = renderer.create(<DiffModal {...props} client={{ diff } as any} />);
  });
  expect(
    tree.root
      .findAllByType(Text)
      .some(t => String(t.props.children).includes('Could not load')),
  ).toBe(true);
  await act(async () =>
    tree.root
      .findAllByType(TouchableOpacity)
      .find(t => t.props.accessibilityRole === 'button')!
      .props.onPress(),
  );
  expect(diff).toHaveBeenCalledTimes(2);
  expect(tree.root.findByType(FlatList).props.data).toEqual(['+recovered']);
});
test('changing a pane retires an earlier read', async () => {
  let finish: (text: string) => void = () => {};
  const diff = jest
    .fn()
    .mockImplementationOnce(
      () =>
        new Promise(r => {
          finish = r;
        }),
    )
    .mockResolvedValueOnce('+current');
  const client = { diff } as any;
  await act(async () => {
    tree = renderer.create(<DiffModal {...props} client={client} />);
  });
  await act(async () =>
    tree.update(<DiffModal {...props} paneId="%next" client={client} />),
  );
  await act(async () => finish('+stale'));
  expect(tree.root.findByType(FlatList).props.data).toEqual(['+current']);
});
test('minified lines have a bounded native text measurement and disclose shortening', () => {
  const result = diffRows('+' + 'x'.repeat(400000));
  expect(result.shortened).toBe(true);
  expect(result.lines[0].length).toBeLessThan(2100);
  expect(result.columns).toBeLessThan(2100);
  expect(diffRows('+small\n context\n').lines).toEqual(['+small', ' context']);
});

test('wide diffs keep a contrasting position indicator in a fixed viewport gutter', async () => {
  const client = {
    diff: async () =>
      Array.from({ length: 200 }, () => '+' + 'x'.repeat(200)).join('\n'),
  } as any;
  await act(async () => {
    tree = renderer.create(
      <DiffModal {...props} pal={paletteFor('light')} client={client} />,
    );
  });
  const horizontal = () =>
    tree.root
      .findAllByType(ScrollView)
      .find(n => n.props.testID === 'diff-horizontal')!;
  const list = () => tree.root.findByType(FlatList);
  act(() => {
    horizontal().props.onLayout({
      nativeEvent: { layout: { width: 300, height: 600 } },
    });
    list().props.onContentSizeChange(1424, 3200);
  });
  const track = tree.root.findByProps({ testID: 'diff-scroll-track' });
  expect(track.props.pointerEvents).toBe('none');
  expect(track.props.style.right).toBe(2);
  expect(horizontal().props.style.marginRight).toBe(8);
  const thumb = tree.root.findByProps({ testID: 'diff-scroll-thumb' });
  const position = () =>
    thumb.props.style[1].transform[0].translateY.__getValue();
  expect(position()).toBe(0);
  act(() =>
    list().props.onScroll({ nativeEvent: { contentOffset: { y: 1300 } } }),
  );
  expect(position()).toBeGreaterThan(0);
  act(() =>
    list().props.onScroll({ nativeEvent: { contentOffset: { y: 2600 } } }),
  );
  expect(position()).toBeCloseTo(592 - thumb.props.style[1].height);
  expect(list().props.style.width).toBeGreaterThan(300);
  expect(list().props.showsVerticalScrollIndicator).toBe(false);
  expect(horizontal().props.indicatorStyle).toBe('white');
  expect(tree.root.findAllByType(Text).length).toBeLessThan(200);
  act(() =>
    horizontal().props.onLayout({
      nativeEvent: { layout: { width: 1800, height: 800 } },
    }),
  );
  expect(
    tree.root.findAllByProps({ testID: 'diff-scroll-track' }).length,
  ).toBeGreaterThan(0);
  act(() => list().props.onContentSizeChange(1800, 100));
  expect(
    tree.root.findAllByProps({ testID: 'diff-scroll-track' }),
  ).toHaveLength(0);
});

test('file cards, hunk folding, line numbers and full source reader share the original diff', async () => {
  const text =
    'diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-package old\n+package current\n@@ -9 +9 @@\n-before\n+after';
  await act(async () => {
    tree = renderer.create(
      <DiffModal {...props} client={{ diff: async () => text } as any} />,
    );
  });
  const list = () => tree.root.findByType(FlatList);
  expect(list().props.data).toContain('+package current');
  expect(list().props.data).not.toContain('+after');
  expect(list().props.getItemLayout).toBeUndefined();
  const row = list().props.renderItem({ item: list().props.data[0], index: 0 });
  let file: renderer.ReactTestRenderer;
  act(() => {
    file = renderer.create(row);
  });
  const open = file!.root
    .findAllByType(TouchableOpacity)
    .find(n => n.props.accessibilityLabel === 'Read complete file changes')!;
  act(() => open.props.onPress());
  const reader = tree.root
    .findAllByProps({ source: text })
    .find(n => n.props.title === 'a.go');
  expect(reader).toBeDefined();
  act(() => file!.unmount());
});
