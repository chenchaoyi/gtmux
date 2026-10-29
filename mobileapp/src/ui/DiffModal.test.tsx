import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {FlatList, Text, TouchableOpacity} from 'react-native';
import {DiffModal} from './DiffModal';
import {paletteFor} from './theme';
import {diffRows} from './diff';
const props = {visible: true, paneId: '%synthetic', pal: paletteFor('dark'), lang: 'en' as const, onClose: jest.fn()};
let tree: renderer.ReactTestRenderer;
afterEach(() => { if (tree) act(() => tree.unmount()); });
test('large diffs mount only a window of native rows and retain the final line', async () => {
  const lines = Array.from({length: 40000}, (_, i) => '+line ' + i);
  await act(async () => { tree = renderer.create(<DiffModal {...props} client={{diff: async () => lines.join('\n')} as any} />); });
  const list = tree.root.findByType(FlatList);
  expect(list.props.data).toHaveLength(40000);
  expect(list.props.data.at(-1)).toBe('+line 39999');
  expect(tree.root.findAllByType(Text).length).toBeLessThan(200);
  expect(list.props.getItemLayout(undefined, 39999).offset).toBeGreaterThan(600000);
});
test('a failed read is an error with retry, not an empty repository', async () => {
  const diff = jest.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce('+recovered');
  await act(async () => { tree = renderer.create(<DiffModal {...props} client={{diff} as any} />); });
  expect(tree.root.findAllByType(Text).some(t => String(t.props.children).includes('Could not load'))).toBe(true);
  await act(async () => tree.root.findAllByType(TouchableOpacity).find(t => t.props.accessibilityRole === 'button')!.props.onPress());
  expect(diff).toHaveBeenCalledTimes(2);
  expect(tree.root.findByType(FlatList).props.data).toEqual(['+recovered']);
});
test('changing a pane retires an earlier read', async () => {
  let finish: (text: string) => void = () => {};
  const diff = jest.fn().mockImplementationOnce(() => new Promise(r => {finish = r;})).mockResolvedValueOnce('+current');
  const client = {diff} as any;
  await act(async () => { tree = renderer.create(<DiffModal {...props} client={client} />); });
  await act(async () => tree.update(<DiffModal {...props} paneId="%next" client={client} />));
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
