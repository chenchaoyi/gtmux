import React from 'react';
import {Modal, ScrollView, StyleSheet, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import Clipboard from '@react-native-clipboard/clipboard';
import {PinnedPrompt, promptLabel} from './PinnedPrompt';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

const pal = paletteFor('dark');
const prompt = '你是独立只读诊断 worker，不是 HQ。任务：核实 HQ 自轮换会话归属，读 hook 和 resume 记录，给出结论。';

const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => { act(() => { mounted.splice(0).forEach(t => t.unmount()); }); });

function mount(p = prompt, onHeight?: (h: number) => void) {
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<PinnedPrompt prompt={p} pal={pal} lang="zh" onHeight={onHeight} />);
  });
  mounted.push(tree);
  return tree;
}
const bar = (t: renderer.ReactTestRenderer) => t.root.findByProps({testID: TestIds.detail.pinnedPrompt});

test('long instructions open in a separate reader; the terminal entrance stays bounded', () => {
  const long = prompt.repeat(500);
  const t = mount(long);
  const before = StyleSheet.flatten(bar(t).props.style);
  expect(before.maxHeight).toBeLessThanOrEqual(64);
  expect(t.root.findAllByType(Modal)).toHaveLength(0);
  act(() => bar(t).props.onPress());
  const modal = t.root.findByType(Modal);
  expect(modal.props.presentationStyle).toBe('pageSheet');
  const full = modal.findAllByType(Text).find(n => n.props.children === long)!;
  expect(full.props.numberOfLines).toBeUndefined();
  expect(full.props.selectable).toBe(true);
  expect(modal.findAllByType(ScrollView)).toHaveLength(1);
  expect(StyleSheet.flatten(bar(t).props.style)).toEqual(before);
  act(() => modal.props.onRequestClose());
  expect(t.root.findAllByType(Modal)).toHaveLength(0);
});

test('a long press copies the full prompt and says so', () => {
  (Clipboard.setString as jest.Mock).mockClear();
  jest.useFakeTimers();
  const t = mount();
  act(() => bar(t).props.onLongPress());
  expect(Clipboard.setString).toHaveBeenCalledWith(prompt);
  expect(t.root.findAllByType(Text).some(n => n.props.children === '已复制')).toBe(true);
  act(() => jest.advanceTimersByTime(2000));
  expect(t.root.findAllByType(Text).some(n => n.props.children === '已复制')).toBe(false);
  jest.useRealTimers();
});

test('a new prompt starts folded again', () => {
  const t = mount();
  act(() => bar(t).props.onPress());
  act(() => t.update(<PinnedPrompt prompt={prompt + '（续）'} pal={pal} lang="zh" />));
  expect(t.root.findAllByType(Modal)).toHaveLength(0);
  expect(bar(t).props.accessibilityState.expanded).toBe(false);
});

test('reports its height so the terminal can make room, and reads the prompt to a screen reader', () => {
  const heights: number[] = [];
  const t = mount(prompt, h => heights.push(h));
  act(() => bar(t).props.onLayout({nativeEvent: {layout: {x: 0, y: 0, width: 390, height: 46}}}));
  expect(heights).toEqual([46]);
  expect(bar(t).props.accessibilityLabel).toBe('本轮提示：' + prompt);
});

describe('the screen reader hears the opening of a long prompt; the bar keeps all of it', () => {
  // Twenty thousand characters of Chinese over many lines, the size of a long dispatch.
  const para = '第一段：核实 HQ 自轮换会话归属，读 hook 和 resume 记录。\n第二段：给出结论和证据。\n\n';
  const long = para.repeat(Math.ceil(20000 / para.length)).slice(0, 20000);

  test('the label is at most 160 characters of the prompt, newlines read as spaces', () => {
    const label = promptLabel(long, true);
    expect(label.startsWith('本轮提示：第一段：核实 HQ 自轮换会话归属')).toBe(true);
    const shown = Array.from(label.slice('本轮提示：'.length));
    expect(shown).toHaveLength(161); // 160 characters and the "…"
    expect(shown[160]).toBe('…');
    expect(label).not.toMatch(/\n/);
    expect(label).toContain('记录。 第二段');
  });

  test('a short prompt is read whole, without a "…", in either language', () => {
    expect(promptLabel('fix the\nbuild', false)).toBe("This turn's prompt: fix the build");
    const exact = '字'.repeat(160);
    expect(promptLabel(exact, true)).toBe('本轮提示：' + exact);
  });

  test('characters outside the BMP are not split in half', () => {
    const label = promptLabel('𠀀'.repeat(200), true);
    expect(Array.from(label.slice('本轮提示：'.length))).toEqual([...Array(160).fill('𠀀'), '…']);
  });

  test('the full instruction remains available in the reader and can be copied there', () => {
    const t = mount(long);
    expect(bar(t).props.accessibilityLabel.length).toBeLessThan(200);
    const full = () => t.root.findAllByType(Text).find(n => n.props.children === long);
    act(() => bar(t).props.onPress());
    expect(full()!.props.numberOfLines).toBeUndefined();
    const copy = t.root.findByType(Modal).findAll(n => n.props.accessibilityRole === 'button' && typeof n.props.onPress === 'function')[0];
    act(() => copy.props.onPress());
    expect(Clipboard.setString).toHaveBeenLastCalledWith(long);
  });
});
