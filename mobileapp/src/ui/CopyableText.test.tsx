import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Platform, StyleSheet, Text, UIManager} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';

jest.spyOn(Platform, 'Version', 'get').mockReturnValue('26.5');
jest.spyOn(UIManager, 'getViewManagerConfig').mockReturnValue({} as any);
jest.mock('react-native/Libraries/ReactNative/requireNativeComponent', () => ({__esModule: true, default: (name: string) => name}));
const {CopyableText, CopyFeedbackContext} = require('./CopyableText');
const {MarkdownView} = require('./MarkdownView');
const trees: renderer.ReactTestRenderer[] = [];
function mount(node: React.ReactNode, enabled = true) {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<CopyFeedbackContext.Provider value={enabled}>{node}</CopyFeedbackContext.Provider>); });
  trees.push(tree);
  return tree;
}
afterEach(() => { act(() => { trees.splice(0).forEach(t => t.unmount()); }); jest.clearAllMocks(); });

test('iOS block copy retains rich text and links, moving only outer layout to the menu wrapper', () => {
  const onPress = jest.fn();
  const tree = mount(<CopyableText selectable lang="zh" style={{flex: 1, marginBottom: 8, fontSize: 15, lineHeight: 22, color: '#fff'}}>
    {'中文 '}<Text style={{fontWeight: '700'}}>bold</Text>{' '}<Text onPress={onPress}>link</Text>{'\nnext 😀'}
  </CopyableText>);
  const menu = tree.root.findByType('CopyTextView' as any);
  expect(menu.props.copyText).toBe('中文 bold link\nnext 😀');
  expect(menu.props.copyLabel).toBe('复制');
  expect(StyleSheet.flatten(menu.props.style)).toEqual({flex: 1, marginBottom: 8});
  const text = tree.root.findAllByType(Text)[0];
  expect(StyleSheet.flatten(text.props.style)).toEqual({fontSize: 15, lineHeight: 22, color: '#fff'});
  expect(text.props.selectable).toBe(false);
  act(() => tree.root.findAllByType(Text).find(t => t.props.onPress === onPress)!.props.onPress());
  expect(onPress).toHaveBeenCalledTimes(1);
});

test('the labelled accessibility action copies exactly the rendered block and keeps other actions', () => {
  const other = jest.fn();
  const tree = mount(<CopyableText selectable accessibilityActions={[{name: 'custom', label: 'Other'}]} onAccessibilityAction={other}>one <Text>two</Text></CopyableText>);
  const text = tree.root.findAllByType(Text)[0];
  expect(text.props.accessibilityActions).toEqual([{name: 'custom', label: 'Other'}, {name: 'copy', label: 'Copy'}]);
  act(() => text.props.onAccessibilityAction({nativeEvent: {actionName: 'copy'}}));
  expect(Clipboard.setString).toHaveBeenCalledWith('one two');
  expect(other).not.toHaveBeenCalled();
  act(() => text.props.onAccessibilityAction({nativeEvent: {actionName: 'custom'}}));
  expect(other).toHaveBeenCalledTimes(1);
});

test('non-chat and non-selectable text keep the original renderer', () => {
  for (const [enabled, selectable] of [[false, true], [true, false]]) {
    const tree = mount(<CopyableText selectable={selectable} selectionColor="blue">original</CopyableText>, enabled);
    expect(tree.root.findAllByType('CopyTextView' as any)).toHaveLength(0);
    expect(tree.root.findAllByType(Text)[0].props.selectable).toBe(selectable);
  }
});

test('every selectable Markdown block supplies its own copy scope, not the complete reply', () => {
  const colors = {text: '#fff', dim: '#aaa', code: '#fff', codeBg: '#222', border: '#555', link: '#0ff'};
  const source = '# Heading\n\nFirst **bold** paragraph\n\nSecond paragraph\n\n- item\n\n> quote\n\n```\ncode\n```\n\n| key | value |\n|---|---|\n| a | b |';
  const tree = mount(<MarkdownView source={source} colors={colors} selectable lang="zh" />);
  const menus = tree.root.findAllByType('CopyTextView' as any);
  expect(menus.map(m => m.props.copyText)).toEqual(['Heading', 'First bold paragraph', 'Second paragraph', 'item', 'quote', 'code', 'key', 'value', 'a', 'b']);
  expect(menus.every(m => m.props.copyLabel === '复制')).toBe(true);
});
