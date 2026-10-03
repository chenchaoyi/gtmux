import React from 'react';
import {ScrollView, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import Clipboard from '@react-native-clipboard/clipboard';
import {PinnedPrompt} from './PinnedPrompt';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

const pal = paletteFor('dark');
const prompt = '你是独立只读诊断 worker，不是 HQ。任务：核实 HQ 自轮换会话归属，读 hook 和 resume 记录，给出结论。';

function mount(p = prompt, onHeight?: (h: number) => void) {
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<PinnedPrompt prompt={p} pal={pal} lang="zh" onHeight={onHeight} />);
  });
  return tree;
}
const bar = (t: renderer.ReactTestRenderer) => t.root.findByProps({testID: TestIds.detail.pinnedPrompt});
const body = (t: renderer.ReactTestRenderer) => t.root.findAllByType(Text).find(n => n.props.children === prompt)!;

test('shows the whole prompt, two lines at rest, and a tap opens and closes it', () => {
  const t = mount();
  expect(body(t).props.numberOfLines).toBe(2);
  expect(t.root.findAllByType(ScrollView)).toHaveLength(0);
  act(() => bar(t).props.onPress());
  expect(body(t).props.numberOfLines).toBeUndefined();
  expect(t.root.findAllByType(ScrollView)).toHaveLength(1); // a very long prompt scrolls inside the bar
  expect(bar(t).props.accessibilityState).toEqual({expanded: true});
  act(() => bar(t).props.onPress());
  expect(body(t).props.numberOfLines).toBe(2);
});

test('a long press copies the full prompt and says so', () => {
  (Clipboard.setString as jest.Mock).mockClear();
  jest.useFakeTimers();
  const t = mount();
  act(() => bar(t).props.onLongPress());
  expect(Clipboard.setString).toHaveBeenCalledWith(prompt);
  expect(t.root.findAllByType(Text).some(n => n.props.children === '已拷贝')).toBe(true);
  act(() => jest.advanceTimersByTime(2000));
  expect(t.root.findAllByType(Text).some(n => n.props.children === '已拷贝')).toBe(false);
  jest.useRealTimers();
});

test('a new prompt starts folded again', () => {
  const t = mount();
  act(() => bar(t).props.onPress());
  act(() => t.update(<PinnedPrompt prompt={prompt + '（续）'} pal={pal} lang="zh" />));
  const next = t.root.findAllByType(Text).find(n => n.props.children === prompt + '（续）')!;
  expect(next.props.numberOfLines).toBe(2);
});

test('reports its height so the terminal can make room, and reads the prompt to a screen reader', () => {
  const heights: number[] = [];
  const t = mount(prompt, h => heights.push(h));
  act(() => bar(t).props.onLayout({nativeEvent: {layout: {x: 0, y: 0, width: 390, height: 46}}}));
  expect(heights).toEqual([46]);
  expect(bar(t).props.accessibilityLabel).toBe('当前提示：' + prompt);
});
